import json
import time
import threading
from typing import Optional, Dict, Any
import websocket
from colorama import Fore, Style, init
import os

init(autoreset=True)


class GameClient:
    def __init__(self, gateway_url: str = "ws://130.193.58.223:8080"):
        self.gateway_url = gateway_url
        self.ws: Optional[websocket.WebSocketApp] = None
        self.access_token: Optional[str] = None
        self.is_creator = False
        self.quiz_type = None
        self.current_question = None
        self.question_start_time = None
        self.connected = False
        self.running = True

    def connect(self, instance_id: Optional[str] = None, access_code: Optional[str] = None):
        if not self.access_token:
            print(f"{Fore.RED}Error: Access token not set{Style.RESET_ALL}")
            return False

        params = f"token={self.access_token}"
        if instance_id:
            params += f"&instance_id={instance_id}"
        elif access_code:
            params += f"&access_code={access_code}"
        else:
            print(f"{Fore.RED}Error: Either instance_id or access_code must be provided{Style.RESET_ALL}")
            return False

        ws_url = f"{self.gateway_url}/ws?{params}"

        print(f"{Fore.CYAN}Connecting to {ws_url}...{Style.RESET_ALL}")

        self.ws = websocket.WebSocketApp(
            ws_url,
            on_message=self._on_message,
            on_error=self._on_error,
            on_close=self._on_close,
            on_open=self._on_open
        )

        ws_thread = threading.Thread(target=self.ws.run_forever)
        ws_thread.daemon = True
        ws_thread.start()

        timeout = 10
        start = time.time()
        while not self.connected and time.time() - start < timeout:
            if not self.ws.keep_running:
                print(f"{Fore.RED}WebSocket connection closed unexpectedly{Style.RESET_ALL}")
                return False
            time.sleep(0.1)

        if not self.connected:
            print(f"{Fore.RED}Connection timed out waiting for 'connected' message{Style.RESET_ALL}")

        return self.connected

    def _on_open(self, ws):
        print(f"{Fore.GREEN}✓ WebSocket connected{Style.RESET_ALL}")

    def _on_message(self, ws, message):
        messages = message.split('\n')

        for msg_str in messages:
            if not msg_str.strip():
                continue

            print(f"DEBUG: Received: {msg_str}")

            try:
                data = json.loads(msg_str)
                msg_type = data.get("type")
                payload = data.get("payload", {})

                if msg_type == "connected":
                    self._handle_connected(payload)
                elif msg_type == "participants_update":
                    self._handle_participants_update(payload)
                elif msg_type == "participants_list":
                    self._handle_participants_list(payload)
                elif msg_type == "quiz_started":
                    self._handle_quiz_started(payload)
                elif msg_type == "question":
                    self._handle_question(payload)
                elif msg_type == "answer_progress":
                    self._handle_answer_progress(payload)
                elif msg_type == "answer_result":
                    self._handle_answer_result(payload)
                elif msg_type == "leaderboard":
                    self._handle_leaderboard(payload)
                elif msg_type == "time_expired":
                    self._handle_time_expired(payload)
                elif msg_type == "waiting_for_creator":
                    self._handle_waiting_for_creator(payload)
                elif msg_type == "quiz_finished":
                    self._handle_quiz_finished(payload)
                elif msg_type == "error":
                    self._handle_error(payload)
                elif msg_type == "pong":
                    pass
                else:
                    print(f"{Fore.YELLOW}Unknown message type: {msg_type}{Style.RESET_ALL}")

            except json.JSONDecodeError as e:
                print(f"{Fore.RED}Failed to parse message: {e}. Message: {msg_str}{Style.RESET_ALL}")

    def _on_error(self, ws, error):
        print(f"{Fore.RED}WebSocket error: {error}{Style.RESET_ALL}")

    def _on_close(self, ws, close_status_code, close_msg):
        self.connected = False
        print(f"{Fore.YELLOW}WebSocket closed (code: {close_status_code}, msg: {close_msg}){Style.RESET_ALL}")

    def _handle_connected(self, payload: Dict[str, Any]):
        self.connected = True
        self.is_creator = payload.get("is_creator", False)
        self.quiz_type = payload.get("quiz_type")
        quiz_status = payload.get("quiz_status")

        print(f"\n{Fore.GREEN}{'='*60}{Style.RESET_ALL}")
        print(f"{Fore.GREEN}Connected to Quiz!{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Session ID: {payload.get('session_id')}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Quiz Type: {self.quiz_type}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Status: {quiz_status}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Role: {'Creator' if self.is_creator else 'Participant'}{Style.RESET_ALL}")
        print(f"{Fore.GREEN}{'='*60}{Style.RESET_ALL}\n")

        if self.is_creator and quiz_status == "waiting":
            print(f"{Fore.YELLOW}You are the creator. Type 'start' to begin the quiz.{Style.RESET_ALL}")

    def _handle_participants_update(self, payload: Dict[str, Any]):
        action = payload.get("action")
        user_id = payload.get("user_id")
        count = payload.get("count")

        if action == "joined":
            print(f"{Fore.GREEN}➕ User {user_id} joined (Total: {count}){Style.RESET_ALL}")
        elif action == "left":
            print(f"{Fore.RED}➖ User {user_id} left (Total: {count}){Style.RESET_ALL}")

    def _handle_participants_list(self, payload: Dict[str, Any]):
        participants = payload.get("participants", [])
        quiz = payload.get("quiz", {})
        quiz_title = quiz.get("title", "Unknown Quiz")

        print(f"\n{Fore.CYAN}{'='*60}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}👥 Participants{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Quiz: {quiz_title}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}{'='*60}{Style.RESET_ALL}")

        for participant in participants:
            user_id = participant.get("user_id")
            first_name = participant.get("first_name", "")
            last_name = participant.get("last_name", "")
            email = participant.get("email", "")
            avatar_url = participant.get("avatar_url", "")
            is_creator = participant.get("is_creator", False)

            name = f"{first_name} {last_name}".strip() or user_id
            role = "👑 Creator" if is_creator else "👤 Participant"

            print(f"\n{Fore.GREEN}{role}{Style.RESET_ALL}")
            print(f"  Name: {name}")
            print(f"  Email: {email}")
            print(f"  User ID: {user_id}")
            if avatar_url:
                print(f"  Avatar: {avatar_url}")

        print(f"{Fore.CYAN}{'='*60}{Style.RESET_ALL}\n")

    def _handle_quiz_started(self, payload: Dict[str, Any]):
        print(f"\n{Fore.GREEN}{'='*60}{Style.RESET_ALL}")
        print(f"{Fore.GREEN}🎮 QUIZ STARTED!{Style.RESET_ALL}")
        print(f"{Fore.GREEN}{'='*60}{Style.RESET_ALL}\n")

    def _handle_question(self, payload: Dict[str, Any]):
        self.current_question = payload.get("question")
        question_index = payload.get("question_index")
        total_questions = payload.get("total_questions")
        time_limit_ms = payload.get("time_limit_ms", 0)
        server_time = payload.get("server_time")

        self.question_start_time = time.time()

        question = self.current_question

        print(f"\n{Fore.CYAN}{'='*60}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Question {question_index + 1}/{total_questions}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}{'='*60}{Style.RESET_ALL}")
        print(f"\n{Fore.WHITE}{Style.BRIGHT}{question['text']}{Style.RESET_ALL}\n")

        q_type = question['type']
        if q_type in ('single', 'multiple') and question.get('options'):
            print(f"{Fore.YELLOW}Options:{Style.RESET_ALL}")
            for i, option in enumerate(question['options']):
                print(f"  {i}. {option}")
            print()

        if time_limit_ms > 0:
            print(f"{Fore.YELLOW}⏱️  Time limit: {time_limit_ms / 1000:.1f} seconds{Style.RESET_ALL}")

        if q_type == 'single':
            print(f"{Fore.GREEN}Enter the option number (0-{len(question.get('options', [])) - 1}):{Style.RESET_ALL} ", end='', flush=True)
        elif q_type == 'multiple':
            print(f"{Fore.GREEN}Enter option numbers separated by commas (e.g. 0,2):{Style.RESET_ALL} ", end='', flush=True)
        else:
            print(f"{Fore.GREEN}Type your answer:{Style.RESET_ALL} ", end='', flush=True)

    def _handle_answer_progress(self, payload: Dict[str, Any]):
        participants_answered = payload.get("participants_answered", 0)
        total_participants = payload.get("total_participants", 0)

        print(f"\r{' ' * 80}", end='', flush=True)
        print(f"\r{Fore.BLUE}📊 Progress: {participants_answered}/{total_participants} participants answered{Style.RESET_ALL}", end='', flush=True)

        if participants_answered == total_participants:
            print(f"\n{Fore.GREEN}✓ All participants have answered!{Style.RESET_ALL}", end='', flush=True)

    def _handle_answer_result(self, payload: Dict[str, Any]):
        is_correct = payload.get("is_correct")
        score = payload.get("score")
        time_spent_ms = payload.get("time_spent_ms")
        total_score = payload.get("total_score")

        print(f"\r{' ' * 80}", end='', flush=True)
        print()

        if is_correct:
            print(f"{Fore.GREEN}✓ Correct! +{score} points{Style.RESET_ALL}")
        else:
            print(f"{Fore.RED}✗ Incorrect{Style.RESET_ALL}")

        print(f"{Fore.CYAN}Time: {time_spent_ms / 1000:.2f}s | Total Score: {total_score}{Style.RESET_ALL}")

    def _handle_leaderboard(self, payload: Dict[str, Any]):
        leaderboard = payload.get("leaderboard", [])
        questions_stats = payload.get("questions_stats", [])

        if leaderboard:
            print(f"\n{Fore.MAGENTA}{'='*60}{Style.RESET_ALL}")
            print(f"{Fore.MAGENTA}🏆 LEADERBOARD{Style.RESET_ALL}")
            print(f"{Fore.MAGENTA}{'='*60}{Style.RESET_ALL}")

            for entry in leaderboard:
                rank = entry.get("rank")
                user = entry.get("user", {})
                user_id = user.get("user_id", entry.get("user_id", "Unknown"))
                score = entry.get("score")
                is_answered = entry.get("is_answered", False)

                medal = ""
                if rank == 1:
                    medal = "1. "
                elif rank == 2:
                    medal = "2. "
                elif rank == 3:
                    medal = "3. "

                user_name = user_id
                if user:
                    first_name = user.get("first_name", "")
                    last_name = user.get("last_name", "")
                    if first_name or last_name:
                        user_name = f"{first_name} {last_name}".strip()

                answered_indicator = f" {Fore.GREEN}✓{Style.RESET_ALL}" if is_answered else f" {Fore.RED}○{Style.RESET_ALL}"

                print(f"{medal} {rank}. {user_name}: {score} points{answered_indicator}")

            print(f"{Fore.MAGENTA}{'='*60}{Style.RESET_ALL}\n")

        if questions_stats:
            print(f"{Fore.CYAN}📊 Answer Statistics:{Style.RESET_ALL}")
            for stat in questions_stats:
                option = stat.get("option", "")
                count = stat.get("count", 0)
                print(f"  {option}: {count} votes")
            print()

    def _handle_time_expired(self, payload: Dict[str, Any]):
        question_index = payload.get("question_index")
        print(f"\r{' ' * 80}", end='', flush=True)
        print(f"\n{Fore.RED}⏰ Time expired for question {question_index + 1}!{Style.RESET_ALL}")

    def _handle_waiting_for_creator(self, payload: Dict[str, Any]):
        question_index = payload.get("question_index")
        reason = payload.get("reason", "")

        print(f"\r{' ' * 80}", end='', flush=True)
        print(f"\n{Fore.YELLOW}⏸️  Waiting for creator to continue...{Style.RESET_ALL}")
        if reason:
            print(f"{Fore.YELLOW}Reason: {reason}{Style.RESET_ALL}")

        if self.is_creator:
            print(f"{Fore.GREEN}Type 'continue' to proceed to the next question.{Style.RESET_ALL}")

    def _handle_quiz_finished(self, payload: Dict[str, Any]):
        final_score = payload.get("final_score")
        rank = payload.get("rank")
        leaderboard = payload.get("leaderboard", [])

        print(f"\n{Fore.GREEN}{'='*60}{Style.RESET_ALL}")
        print(f"{Fore.GREEN}QUIZ FINISHED!{Style.RESET_ALL}")
        print(f"{Fore.GREEN}{'='*60}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Final Score: {final_score}{Style.RESET_ALL}")
        print(f"{Fore.CYAN}Your Rank: {rank}{Style.RESET_ALL}")

        if leaderboard:
            print(f"\n{Fore.MAGENTA}{'='*60}{Style.RESET_ALL}")
            print(f"{Fore.MAGENTA}FINAL LEADERBOARD{Style.RESET_ALL}")
            print(f"{Fore.MAGENTA}{'='*60}{Style.RESET_ALL}")

            for entry in leaderboard:
                rank_entry = entry.get("rank")
                user = entry.get("user", {})
                user_id = user.get("user_id", entry.get("user_id", "Unknown"))
                score = entry.get("score")
                is_answered = entry.get("is_answered", False)

                medal = ""
                if rank_entry == 1:
                    medal = "1. "
                elif rank_entry == 2:
                    medal = "2. "
                elif rank_entry == 3:
                    medal = "3. "

                user_name = user_id
                if user:
                    first_name = user.get("first_name", "")
                    last_name = user.get("last_name", "")
                    if first_name or last_name:
                        user_name = f"{first_name} {last_name}".strip()

                answered_indicator = f" {Fore.GREEN}✓{Style.RESET_ALL}" if is_answered else f" {Fore.RED}○{Style.RESET_ALL}"

                print(f"{medal} {rank_entry}. {user_name}: {score} points{answered_indicator}")

            print(f"{Fore.MAGENTA}{'='*60}{Style.RESET_ALL}")

        print(f"{Fore.GREEN}{'='*60}{Style.RESET_ALL}\n")

    def _handle_error(self, payload: Dict[str, Any]):
        message = payload.get("message", "Unknown error")
        print(f"{Fore.RED}Error: {message}{Style.RESET_ALL}")

    def send_message(self, msg_type: str, payload: Optional[Dict[str, Any]] = None):
        if not self.ws or not self.connected:
            print(f"{Fore.RED}Not connected to server{Style.RESET_ALL}")
            return False

        message = {
            "type": msg_type,
            "payload": payload or {}
        }

        try:
            self.ws.send(json.dumps(message))
            return True
        except Exception as e:
            print(f"{Fore.RED}Failed to send message: {e}{Style.RESET_ALL}")
            return False

    def start_quiz(self):
        if not self.is_creator:
            print(f"{Fore.RED}Only the creator can start the quiz{Style.RESET_ALL}")
            return False

        return self.send_message("start_quiz")

    def submit_answer(self, answer: str):
        if not self.current_question:
            print(f"{Fore.RED}No active question{Style.RESET_ALL}")
            return False

        time_spent_ms = int((time.time() - self.question_start_time) * 1000) if self.question_start_time else 0

        q_type = self.current_question.get("type")
        formatted_answer = answer

        if q_type == "multiple":
            try:
                indices = [int(x.strip()) for x in answer.split(",")]
                formatted_answer = json.dumps(indices)
            except ValueError:
                print(f"{Fore.RED}Invalid format. Enter numbers separated by commas (e.g. 0,2){Style.RESET_ALL}")
                return False

        payload = {
            "question_id": self.current_question["id"],
            "answer": formatted_answer,
            "time_spent_ms": time_spent_ms
        }

        return self.send_message("answer", payload)

    def continue_quiz(self):
        if not self.is_creator:
            print(f"{Fore.RED}Only the creator can continue the quiz{Style.RESET_ALL}")
            return False

        return self.send_message("continue")

    def kick_participant(self, email: str):
        if not self.is_creator:
            print(f"{Fore.RED}Only the creator can kick participants{Style.RESET_ALL}")
            return False

        if self.quiz_type != "sync":
            print(f"{Fore.RED}Kick is only available in sync quizzes{Style.RESET_ALL}")
            return False

        return self.send_message("kick", {"email": email})

    def disconnect(self):
        self.running = False
        if self.ws:
            self.ws.close()
        print(f"{Fore.YELLOW}Disconnected{Style.RESET_ALL}")


def main():
    print(f"{Fore.CYAN}{'='*60}{Style.RESET_ALL}")
    print(f"{Fore.CYAN}Kollocol Quiz Game Client{Style.RESET_ALL}")
    print(f"{Fore.CYAN}{'='*60}{Style.RESET_ALL}\n")

    gateway_url = "ws://130.193.58.223:8080"

    client = GameClient(gateway_url)

    access_token = os.getenv("ACCESS_TOKEN")
    if not access_token:
        print(f"{Fore.RED}Access token is required{Style.RESET_ALL}")
        return

    client.access_token = access_token

    print(f"\n{Fore.YELLOW}Connect using:{Style.RESET_ALL}")
    print("1. Instance ID")
    print("2. Access Code")
    choice = input(f"{Fore.GREEN}Choice (1/2): {Style.RESET_ALL}").strip()

    instance_id = None
    access_code = None

    if choice == "1":
        instance_id = input(f"{Fore.GREEN}Instance ID: {Style.RESET_ALL}").strip()
    elif choice == "2":
        access_code = input(f"{Fore.GREEN}Access Code (6 digits): {Style.RESET_ALL}").strip()
    else:
        print(f"{Fore.RED}Invalid choice{Style.RESET_ALL}")
        return

    if not client.connect(instance_id=instance_id, access_code=access_code):
        print(f"{Fore.RED}Failed to connect{Style.RESET_ALL}")
        return

    print(f"\n{Fore.CYAN}Commands:{Style.RESET_ALL}")
    print("  start           - Start the quiz (creator only)")
    print("  continue        - Continue to next question (creator only)")
    print("  kick <email>    - Kick a participant by email (creator only, sync quiz only)")
    print("  quit            - Disconnect and exit")
    print("  <number>        - Answer for single-choice (option index, e.g. 0)")
    print("  <n,n,...>       - Answer for multiple-choice (indices, e.g. 0,2)")
    print("  <text>          - Answer for open question\n")

    try:
        while client.running and client.connected:
            try:
                user_input = input().strip()

                if not user_input:
                    continue

                if user_input.lower() == "quit":
                    break
                elif user_input.lower() == "start":
                    client.start_quiz()
                elif user_input.lower() == "continue":
                    client.continue_quiz()
                elif user_input.lower().startswith("kick "):
                    email = user_input[5:].strip()
                    if email:
                        client.kick_participant(email)
                    else:
                        print(f"{Fore.RED}Please provide an email address to kick{Style.RESET_ALL}")
                else:
                    client.submit_answer(user_input)

            except EOFError:
                break

    except KeyboardInterrupt:
        print(f"\n{Fore.YELLOW}Interrupted by user{Style.RESET_ALL}")
    finally:
        client.disconnect()


if __name__ == "__main__":
    main()
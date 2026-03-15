import json
import logging

import grpc
from openai import OpenAI

import ml_pb2
import ml_pb2_grpc
from prompts import (
    GENERATE_QUESTIONS_SYSTEM,
    GENERATE_TEMPLATE_SYSTEM,
    PARAPHRASE_SYSTEM,
)

logger = logging.getLogger(__name__)


class MLServiceServicer(ml_pb2_grpc.MLServiceServicer):
    def __init__(self, api_key: str, folder: str, model: str):
        self.client = OpenAI(
            api_key=api_key,
            base_url="https://ai.api.cloud.yandex.net/v1",
        )
        self.model = f"gpt://{folder}/{model}"

    def _chat(self, system: str, user: str) -> str:
        response = self.client.chat.completions.create(
            model=self.model,
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
            temperature=0.7,
        )
        return response.choices[0].message.content.strip()

    def Paraphrase(self, request, context):
        if not request.text:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("text is required")
            return ml_pb2.ParaphraseResponse()

        try:
            result = self._chat(PARAPHRASE_SYSTEM, request.text)
            return ml_pb2.ParaphraseResponse(text=result)
        except Exception as e:
            logger.error("Paraphrase failed: %s", e)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return ml_pb2.ParaphraseResponse()

    def GenerateTemplate(self, request, context):
        if not request.text:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("text is required")
            return ml_pb2.GenerateTemplateResponse()

        try:
            raw = self._chat(GENERATE_TEMPLATE_SYSTEM, request.text)
            data = json.loads(raw.strip("```"))

            questions = []
            for q in data.get("questions", []):
                questions.append(self._dict_to_generated_question(q))

            return ml_pb2.GenerateTemplateResponse(
                title=data.get("title", ""),
                questions=questions,
            )
        except json.JSONDecodeError as e:
            logger.error("GenerateTemplate JSON parse failed: %s\nRaw: %s", e, raw)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Failed to parse LLM response as JSON: {e}")
            return ml_pb2.GenerateTemplateResponse()
        except Exception as e:
            logger.error("GenerateTemplate failed: %s", e)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return ml_pb2.GenerateTemplateResponse()

    def GenerateQuestions(self, request, context):
        existing = []
        for q in request.questions:
            qtype = "single"
            if q.HasField("single_choice"):
                qtype = "single"
            elif q.HasField("multiple_choice"):
                qtype = "multiple"
            elif q.HasField("open_answer"):
                qtype = "open"
            existing.append({
                "text": q.text,
                "type": qtype,
            })

        user_message = ""
        if request.text:
            user_message += f"Текст/тема: {request.text}\n\n"
        if existing:
            user_message += f"Существующие вопросы:\n{json.dumps(existing, ensure_ascii=False)}"

        if not user_message:
            context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
            context.set_details("text or questions are required")
            return ml_pb2.GenerateQuestionsResponse()

        try:
            raw = self._chat(GENERATE_QUESTIONS_SYSTEM, user_message)
            data = json.loads(raw.strip("```"))

            questions = []
            for q in data.get("questions", []):
                questions.append(self._dict_to_generated_question(q))

            return ml_pb2.GenerateQuestionsResponse(questions=questions)
        except json.JSONDecodeError as e:
            logger.error("GenerateQuestions JSON parse failed: %s\nRaw: %s", e, raw)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Failed to parse LLM response as JSON: {e}")
            return ml_pb2.GenerateQuestionsResponse()
        except Exception as e:
            logger.error("GenerateQuestions failed: %s", e)
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(str(e))
            return ml_pb2.GenerateQuestionsResponse()

    @staticmethod
    def _dict_to_generated_question(q: dict) -> ml_pb2.GeneratedQuestion:
        qtype = q.get("type", "single")
        gq = ml_pb2.GeneratedQuestion(
            text=q.get("text", ""),
            max_score=q.get("max_score", 1),
            time_limit_sec=q.get("time_limit_sec", 30),
        )

        if qtype == "single":
            gq.single_choice.CopyFrom(ml_pb2.MLSingleChoice(
                options=q.get("options", []),
                correct_option=q.get("correct_option", 0),
            ))
        elif qtype == "multiple":
            gq.multiple_choice.CopyFrom(ml_pb2.MLMultipleChoice(
                options=q.get("options", []),
                correct_options=q.get("correct_options", []),
            ))
        elif qtype == "open":
            gq.open_answer.CopyFrom(ml_pb2.MLOpenAnswer(
                correct_text=q.get("correct_text", ""),
            ))

        return gq

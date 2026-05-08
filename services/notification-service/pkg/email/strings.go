package email

import (
	"fmt"

	"notification-service/pkg/lang"
)

const (
	TmplAuthCode         = "auth_code"
	TmplGroupInvite      = "group_invite"
	TmplGroupKicked      = "group_kicked"
	TmplQuizCreated      = "quiz_created"
	TmplQuizResults      = "quiz_results"
	TmplGradeChanged     = "grade_changed"
	TmplDeadlineReminder = "deadline_reminder"
)

var subjects = map[lang.Lang]map[string]string{
	lang.RU: {
		TmplAuthCode:         "Kollocol — Код подтверждения",
		TmplGroupInvite:      "Kollocol — Приглашение в группу %s",
		TmplGroupKicked:      "Kollocol — Вас исключили из группы %s",
		TmplQuizCreated:      "Kollocol — Новый тест: %s",
		TmplQuizResults:      "Kollocol — Результаты теста %s (%d/%d)",
		TmplGradeChanged:     "Kollocol — Оценка обновлена: %s (%d/%d)",
		TmplDeadlineReminder: "Kollocol — Напоминание: %s, осталось %s",
	},
	lang.EN: {
		TmplAuthCode:         "Kollocol - Your Verification Code",
		TmplGroupInvite:      "Kollocol - Invitation to join %s",
		TmplGroupKicked:      "Kollocol - You were removed from %s",
		TmplQuizCreated:      "Kollocol - New Quiz: %s",
		TmplQuizResults:      "Kollocol - Results for %s (%d/%d)",
		TmplGradeChanged:     "Kollocol - Grade Updated: %s (%d/%d)",
		TmplDeadlineReminder: "Kollocol - Reminder: %s due in %s",
	},
}

var inAppTitles = map[lang.Lang]map[string]string{
	lang.RU: {
		TmplGroupInvite:      "Приглашение в группу",
		TmplGroupKicked:      "Исключение из группы",
		TmplQuizCreated:      "Новый тест",
		TmplQuizResults:      "Результаты теста",
		TmplGradeChanged:     "Оценка обновлена",
		TmplDeadlineReminder: "Срок прохождения теста",
	},
	lang.EN: {
		TmplGroupInvite:      "Group Invitation",
		TmplGroupKicked:      "Removed from Group",
		TmplQuizCreated:      "New Quiz Available",
		TmplQuizResults:      "Quiz Results Ready",
		TmplGradeChanged:     "Grade Updated",
		TmplDeadlineReminder: "Quiz Deadline Approaching",
	},
}

var inAppContent = map[lang.Lang]map[string]string{
	lang.RU: {
		TmplGroupInvite:      "%s пригласил(а) вас в группу %s",
		TmplGroupKicked:      "%s исключил(а) вас из группы %s",
		TmplDeadlineReminder: "%s — осталось %s",
	},
	lang.EN: {
		TmplGroupInvite:      "%s invited you to %s",
		TmplGroupKicked:      "%s removed you from %s",
		TmplDeadlineReminder: "%s — %s remaining",
	},
}

func tr(table map[lang.Lang]map[string]string, l lang.Lang, key string) string {
	if m, ok := table[l]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	if m, ok := table[lang.Default]; ok {
		if v, ok := m[key]; ok {
			return v
		}
	}
	return key
}

func Subject(l lang.Lang, key string, args ...any) string {
	return fmt.Sprintf(tr(subjects, l, key), args...)
}

func InAppTitle(l lang.Lang, key string) string {
	return tr(inAppTitles, l, key)
}

func InAppContent(l lang.Lang, key string, args ...any) string {
	return fmt.Sprintf(tr(inAppContent, l, key), args...)
}

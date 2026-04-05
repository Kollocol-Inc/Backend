package websocket

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func newTestHub() *Hub {
	return &Hub{}
}

func TestValidateAnswer_SingleChoice_Correct(t *testing.T) {
	h := newTestHub()
	assert.True(t, h.validateAnswer("1", `"1"`, "single"))
}

func TestValidateAnswer_SingleChoice_Wrong(t *testing.T) {
	h := newTestHub()
	assert.False(t, h.validateAnswer("2", `"1"`, "single"))
}

func TestValidateAnswer_SingleChoice_TrimSpaces(t *testing.T) {
	h := newTestHub()
	assert.True(t, h.validateAnswer(" 1 ", `"1"`, "single"))
}

func TestValidateAnswer_MultipleChoice_Correct(t *testing.T) {
	h := newTestHub()
	userAnswer, _ := json.Marshal([]int{0, 2})
	correctAnswer, _ := json.Marshal([]int{2, 0})
	assert.True(t, h.validateAnswer(string(userAnswer), string(correctAnswer), "multiple"))
}

func TestValidateAnswer_MultipleChoice_Wrong(t *testing.T) {
	h := newTestHub()
	userAnswer, _ := json.Marshal([]int{0, 1})
	correctAnswer, _ := json.Marshal([]int{0, 2})
	assert.False(t, h.validateAnswer(string(userAnswer), string(correctAnswer), "multiple"))
}

func TestValidateAnswer_MultipleChoice_DifferentLength(t *testing.T) {
	h := newTestHub()
	userAnswer, _ := json.Marshal([]int{0})
	correctAnswer, _ := json.Marshal([]int{0, 2})
	assert.False(t, h.validateAnswer(string(userAnswer), string(correctAnswer), "multiple"))
}

func TestValidateAnswer_MultipleChoice_InvalidJSON(t *testing.T) {
	h := newTestHub()
	assert.False(t, h.validateAnswer("not-json", `[0,1]`, "multiple"))
}

func TestValidateAnswer_OpenAnswer_CaseInsensitive(t *testing.T) {
	h := newTestHub()
	assert.True(t, h.validateAnswer("Hello World", `"hello world"`, "open"))
}

func TestValidateAnswer_OpenAnswer_TrimSpaces(t *testing.T) {
	h := newTestHub()
	assert.True(t, h.validateAnswer("  hello  ", `"hello"`, "open"))
}

func TestValidateAnswer_OpenAnswer_Wrong(t *testing.T) {
	h := newTestHub()
	assert.False(t, h.validateAnswer("wrong", `"correct"`, "open"))
}

func TestValidateAnswer_UnquotedCorrectAnswer(t *testing.T) {
	h := newTestHub()
	assert.True(t, h.validateAnswer("1", "1", "single"))
}

func TestCalculateScore_InstantAnswer(t *testing.T) {
	h := newTestHub()
	score := h.calculateScore(10, 0, 30000)
	assert.Equal(t, 1000, score)
}

func TestCalculateScore_HalfTime(t *testing.T) {
	h := newTestHub()
	score := h.calculateScore(10, 15000, 30000)
	assert.Equal(t, 750, score)
}

func TestCalculateScore_FullTime(t *testing.T) {
	h := newTestHub()
	score := h.calculateScore(10, 30000, 30000)
	assert.Equal(t, 500, score)
}

func TestCalculateScore_OverTime(t *testing.T) {
	h := newTestHub()
	score := h.calculateScore(10, 60000, 30000)
	assert.Equal(t, 500, score)
}

func TestCalculateScore_ZeroTimeLimit(t *testing.T) {
	h := newTestHub()
	score := h.calculateScore(10, 5000, 0)
	assert.Equal(t, 1000, score)
}

func TestCalculateScore_ZeroMaxScore(t *testing.T) {
	h := newTestHub()
	score := h.calculateScore(0, 5000, 30000)
	assert.Equal(t, 0, score)
}

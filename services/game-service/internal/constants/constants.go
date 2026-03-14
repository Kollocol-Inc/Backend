package constants

const (
	InstanceStatusWaiting  = "waiting"  // Sync quiz: waiting for participants to join
	InstanceStatusActive   = "active"   // Quiz is in progress (sync: started, async: available)
	InstanceStatusPendingReview = "pending_review" // Quiz has ended by author did not review results
	InstanceStatusReviewed = "reviewed" // Quiz has ended and been reviewed
)

const (
	SessionStatusNotStarted = "not_started" // User has not joined
	SessionStatusJoined     = "joined"      // User has joined but quiz hasn't started
	SessionStatusInProgress = "in_progress" // User is actively playing
	SessionStatusFinished   = "finished"    // User has completed the quiz
)

const (
	QuizTypeSync  = "sync"
	QuizTypeAsync = "async"
)

const (
	ActionJoined = "joined"
	ActionLeft   = "left"
)

const (
	QuestionTypeSingle   = "single"
	QuestionTypeMultiple = "multiple"
	QuestionTypeOpen     = "open"
)
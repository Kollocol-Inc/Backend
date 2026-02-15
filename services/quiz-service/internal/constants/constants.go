package constants

const (
	InstanceStatusWaiting  = "waiting"  // Sync quiz: waiting for participants to join
	InstanceStatusActive   = "active"   // Quiz is in progress (sync: started, async: available)
	InstanceStatusFinished = "finished" // Quiz has ended
)

const (
	SessionStatusNotStarted = "not_started" // User is in group but hasn't joined the quiz yet
	SessionStatusJoined     = "joined"      // User has joined but quiz hasn't started
	SessionStatusInProgress = "in_progress" // User is actively playing
	SessionStatusFinished   = "finished"    // User has completed the quiz
)

const (
	QuizTypeSync  = "sync"
	QuizTypeAsync = "async"
)
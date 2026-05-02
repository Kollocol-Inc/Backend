package websocket

import (
	"testing"

	pb "game-service/proto"
)

func TestUserFromProfile_NilProfile(t *testing.T) {
	got := userFromProfile(nil, true)

	if got.UserID != "" || got.FirstName != "" || got.Email != "" {
		t.Errorf("nil profile should yield empty fields, got %+v", got)
	}
	if !got.IsCreator {
		t.Errorf("IsCreator must be preserved: got %+v", got)
	}
	if !got.IsOnline {
		t.Errorf("IsOnline must be true for a freshly-joined user: got %+v", got)
	}
}

func TestUserFromProfile_PopulatesFields(t *testing.T) {
	p := &pb.User{
		Id:        "u-1",
		FirstName: "Alice",
		LastName:  "Liddell",
		Email:     "a@example.com",
		AvatarUrl: "https://x/y.png",
	}

	got := userFromProfile(p, false)

	if got.UserID != "u-1" || got.FirstName != "Alice" || got.LastName != "Liddell" ||
		got.Email != "a@example.com" || got.AvatarURL != "https://x/y.png" {
		t.Errorf("fields not copied correctly: %+v", got)
	}
	if got.IsCreator {
		t.Errorf("IsCreator must be false")
	}
	if !got.IsOnline {
		t.Errorf("IsOnline must be true")
	}
}

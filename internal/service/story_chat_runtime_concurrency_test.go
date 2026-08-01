package service

import "testing"

func TestStoryChatRuntimePerChatTurnLock(t *testing.T) {
	runtime := &StoryChatRuntime{}
	if !runtime.beginTurn("chat-1") {
		t.Fatal("first turn should acquire lock")
	}
	if runtime.beginTurn("chat-1") {
		t.Fatal("second turn should be rejected")
	}
	if !runtime.beginTurn("chat-2") {
		t.Fatal("different chat should be allowed")
	}
	runtime.finishTurn("chat-1")
	if !runtime.beginTurn("chat-1") {
		t.Fatal("lock should be released")
	}
	runtime.finishTurn("chat-2")
	runtime.finishTurn("chat-1")
}

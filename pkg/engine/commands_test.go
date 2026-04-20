package engine

import (
	"testing"
)

func TestCommandRegistry(t *testing.T) {
	c := NewContest(16000)
	reg := DefaultRegistry()

	t.Run("WPM Command", func(t *testing.T) {
		err := reg.Execute(c, "wpm 35")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if c.MyWpm != 35 {
			t.Errorf("Expected WPM 35, got %d", c.MyWpm)
		}
	})

	t.Run("Invalid WPM", func(t *testing.T) {
		err := reg.Execute(c, "wpm 100")
		if err == nil {
			t.Fatal("Expected error for out-of-range WPM, got nil")
		}
	})

	t.Run("Unknown Command", func(t *testing.T) {
		err := reg.Execute(c, "nonexistent")
		if err == nil {
			t.Fatal("Expected error for unknown command, got nil")
		}
	})

	t.Run("Case Insensitivity", func(t *testing.T) {
		err := reg.Execute(c, "WPM 40")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if c.MyWpm != 40 {
			t.Errorf("Expected WPM 40, got %d", c.MyWpm)
		}
	})

	t.Run("Multiple Arguments", func(t *testing.T) {
		err := reg.Execute(c, "tx cq de w7sst")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		// Logic check: UserEnv should be populated
		if c.UserEnv == nil {
			t.Error("Expected UserEnv to be populated after TX")
		}
	})
}

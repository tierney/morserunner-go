package engine

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Handler defines the function signature for a command execution.
type Handler func(c *Contest, args []string) error

// Command represents a single engine command.
type Command struct {
	Name        string
	Description string
	Action      Handler
}

// Registry manages a collection of commands.
type Registry struct {
	commands map[string]Command
}

// NewRegistry creates a new Command Registry.
func NewRegistry() *Registry {
	return &Registry{
		commands: make(map[string]Command),
	}
}

// Register adds a command to the registry.
func (r *Registry) Register(cmd Command) {
	r.commands[strings.ToLower(cmd.Name)] = cmd
}

// Execute parses an input string and executes the matching command.
func (r *Registry) Execute(c *Contest, input string) error {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	parts := strings.Split(input, " ")
	return r.ExecuteCommand(c, parts[0], parts[1:])
}

// ExecuteCommand executes a command by name with provided arguments.
func (r *Registry) ExecuteCommand(c *Contest, name string, args []string) error {
	cmd, ok := r.commands[strings.ToLower(name)]
	if !ok {
		return fmt.Errorf("unknown command: %s", name)
	}

	return cmd.Action(c, args)
}

// List returns all registered commands and their descriptions.
func (r *Registry) List() []Command {
	cmds := make([]Command, 0, len(r.commands))
	for _, cmd := range r.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// DefaultRegistry returns a registry pre-populated with standard engine commands.
func DefaultRegistry() *Registry {
	reg := NewRegistry()

	reg.Register(Command{
		Name:        "wpm",
		Description: "Set CW speed (15-50)",
		Action: func(c *Contest, args []string) error {
			if len(args) < 1 {
				return errors.New("usage: wpm <n>")
			}
			wpm, err := strconv.Atoi(args[0])
			if err != nil || wpm < 15 || wpm > 50 {
				return errors.New("WPM must be between 15 and 50")
			}
			c.MyWpm = wpm
			c.Keyer.SetWpm(wpm, wpm)
			return nil
		},
	})

	reg.Register(Command{
		Name:        "pitch",
		Description: "Set sidetone pitch in Hz",
		Action: func(c *Contest, args []string) error {
			if len(args) < 1 {
				return errors.New("usage: pitch <hz>")
			}
			pitch, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return err
			}
			c.Mixer.Pitch = pitch
			return nil
		},
	})

	reg.Register(Command{
		Name:        "bw",
		Description: "Set filter bandwidth in Hz",
		Action: func(c *Contest, args []string) error {
			if len(args) < 1 {
				return errors.New("usage: bw <hz>")
			}
			bw, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return err
			}
			c.Bandwidth = bw
			c.Mixer.UpdateFilter(bw)
			return nil
		},
	})

	reg.Register(Command{
		Name:        "noise",
		Description: "Set background noise level (0.0 - 1.0)",
		Action: func(c *Contest, args []string) error {
			if len(args) < 1 {
				return errors.New("usage: noise <level>")
			}
			level, err := strconv.ParseFloat(args[0], 64)
			if err != nil {
				return err
			}
			c.NoiseLevel = level
			return nil
		},
	})

	reg.Register(Command{
		Name:        "pileup",
		Description: "Start a pile-up with N stations",
		Action: func(c *Contest, args []string) error {
			count := 5
			if len(args) > 0 {
				if v, err := strconv.Atoi(args[0]); err == nil {
					count = v
				}
			}
			c.StartPileup(count)
			return nil
		},
	})

	reg.Register(Command{
		Name:        "stop",
		Description: "Stop all active stations and tones",
		Action: func(c *Contest, args []string) error {
			c.Stations = nil
			c.TestTone = false
			c.UserEnv = nil
			return nil
		},
	})

	reg.Register(Command{
		Name:        "tx",
		Description: "Send a CW message",
		Action: func(c *Contest, args []string) error {
			if len(args) < 1 {
				return errors.New("usage: tx <message>")
			}
			msg := strings.Join(args, " ")
			c.ProcessUserTX(msg)
			return nil
		},
	})

	return reg
}

package fsm

import (
	"context"
	"fmt"
	"slices"
)

type Transition struct {
	Fsm  *FSM
	To   string
	From []string
}

type Transitions struct {
	Name, To string
	From     []string
}

type (
	Callback func(context.Context, *Transition)
	Actions  struct {
		Callback Callback
		To       string
	}
)

type FSM struct {
	transitions map[string]Transition
	actions     map[string]Callback
	state       string
}

func Build(initial string, transitions []Transitions, actions []Actions) *FSM {
	fsm := FSM{state: initial, transitions: make(map[string]Transition), actions: make(map[string]Callback)}

	for _, transition := range transitions {
		fsm.transitions[transition.Name] = Transition{Fsm: &fsm, To: transition.To, From: transition.From}
	}

	for _, action := range actions {
		fsm.actions[action.To] = action.Callback
	}

	return &fsm
}

func (fsm *FSM) Current() string {
	return fsm.state
}

func (fsm *FSM) Transition(ctx context.Context, name string) error {
	transition, ok := fsm.transitions[name]
	if !ok {
		return fmt.Errorf("transition with name: %s not found", name)
	}

	if !slices.Contains(transition.From, "*") && !slices.Contains(transition.From, fsm.state) {
		return fmt.Errorf("cannot transition from %s to %s", fsm.state, transition.To)
	}

	fsm.state = transition.To
	if action, ok := fsm.actions[transition.To]; ok {
		action(ctx, &transition)
	}

	return nil
}

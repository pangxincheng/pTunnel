package p2p

import "fmt"

type FSM struct {
	glbVar    *interface{}
	states    map[int]func(*interface{}) int
	statesStr map[int]string
}

func NewFSM(glbVar *interface{}) *FSM {
	return &FSM{
		states:    make(map[int]func(*interface{}) int),
		statesStr: make(map[int]string),
		glbVar:    glbVar,
	}
}

func (fsm *FSM) AddState(state int, stateStr string, action func(*interface{}) int) {
	fsm.states[state] = action
	fsm.statesStr[state] = stateStr
}

func (fsm *FSM) Run(state int) {
	for {
		fmt.Println("State:", fsm.statesStr[state])
		action, ok := fsm.states[state]
		if !ok {
			break
		}
		state = action(fsm.glbVar)
	}
}

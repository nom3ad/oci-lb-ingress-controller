package manager

import (
	"context"

	"go.uber.org/zap"
)

type ActionVerb string

var (
	CreateAction ActionVerb = "create"
	UpdateAction ActionVerb = "update"
	DeleteAction ActionVerb = "delete"
)

type action struct {
	done    bool
	verb    ActionVerb
	subject interface{}
	fn      func() error
}

func (a action) do() error {
	defer func() {
		a.done = true
	}()
	if a.done {
		panic("shouldn't be called a finished action twice")
	}
	return a.fn()
}

type ActionDispacther struct {
	ctx      context.Context
	logger   *zap.SugaredLogger
	_actions []action
}

func (ad *ActionDispacther) allSubjects() []interface{} {
	var subjects []interface{}
	for _, a := range ad._actions {
		for _, s := range subjects {
			if a.subject == s {
				goto end
			}
		}
		subjects = append(subjects, a.subject)
	end:
	}
	return subjects
}

func (ad *ActionDispacther) Context() context.Context {
	return ad.ctx
}

func (ad *ActionDispacther) Logger() *zap.SugaredLogger {
	return ad.logger
}

func (ad *ActionDispacther) Run(verb ActionVerb, subjects ...interface{}) error {
	if (len(subjects)) == 0 {
		subjects = ad.allSubjects()
	}
	for _, sub := range subjects {
		for _, action := range ad._actions {
			if action.done || action.verb != verb || action.subject != sub {
				continue
			}
			err := action.do()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (ad *ActionDispacther) AddFunc(verb ActionVerb, subject interface{}, fn func() error) {
	ad._actions = append(ad._actions, (action{
		verb:    verb,
		subject: subject,
		fn:      fn,
	}))
}

package main

import (
	"testing"
)

var (
	templateProgram = []rule{
		{
			head: process{functor: "sum", args: []expression{
				variable(0), variable(1),
			}},
			body: []process{
				{functor: "sum1", args: []expression{
					variable(0), number(0), variable(1),
				}},
			},
		},
		{
			head: process{functor: "sum1", args: []expression{
				list{head: variable(0), tail: variable(1)}, variable(2), variable(3),
			}},
			body: []process{
				// todo: complex expressions / infix operators that include processes ( is(A1, +(A, X)) )
				{functor: "isplus", args: []expression{
					variable(4), variable(2), variable(0),
				}},
				{functor: "sum1", args: []expression{
					variable(1), variable(4), variable(3),
				}},
			},
		},
		{
			head: process{functor: "sum1", args: []expression{
				emptylist, variable(0), variable(1),
			}},
			body: []process{
				{functor: ":=", args: []expression{
					variable(1), variable(0),
				}},
			},
		},
	}
)

func TestCMatch(t *testing.T) {
	base := bindings{}
	p := process{functor: "sum", args: []expression{
		list{head: number(1), tail: variable(0)}, variable(1),
	}}
	r := rule{head: process{functor: "sum", args: []expression{
		variable(2), variable(3),
	}},
		body: []process{
			{functor: "sum1", args: []expression{
				variable(2), number(0), variable(3),
			}},
		},
	}
	ok, theta, _ := cmatch(base, p, r)
	if !ok {
		t.Fatalf("expected succesful cmatch but got failure")
	}
	want2 := list{head: number(1), tail: variable(0)}
	want3 := variable(1)
	if len(theta) != 2 || theta[variable(2)] != want2 || theta[variable(3)] != want3 {
		t.Fatalf("expected var bindings 2=[1|v#0], 3=v#1 but got %v", theta)
	}
}

func TestInterpretSingleThreaded(t *testing.T) {
	i := NewSingleThreadedInterpreter(templateProgram)
	l, r := i.fresh(), i.fresh()
	q := []process{
		{functor: "sum", args: []expression{
			list{head: number(1), tail: l}, r,
		}},
		{functor: ":=", args: []expression{
			l, list{head: number(2), tail: list{head: number(3), tail: emptylist}},
		}},
	}
	res, deadlocked := i.interpretSinglethreaded(q)
	if deadlocked {
		t.Fatalf("deadlocked!")
	}
	got := walk(res, r)
	// todo: very seldomly I get this error:
	// interpreter_test.go:88: expected 6 but got %!s(main.variable=20)
	if got != number(6) {
		t.Fatalf("expected 6 but got %s", got)
	}
}

func TestInterpretSingleThreadedDeadlockOnMatch(t *testing.T) {
	s := MustParseRules(`
    member(X,[X1|Rest],R) :-
        X =\= X1 | member(X,Rest,R).
    member(X,[X1|_],R) :-
        X == X1 | R := true.
    member(_, [], R) :- R := false.`)
	i := NewSingleThreadedInterpreter(s)
	// this would work in Prolog, but not in FGHC (suspends on X)
	q, _ := i.MustParseProcesses("member(X, [1,2,3], R)")
	res, deadlocked := i.interpretSinglethreaded(q)
	if !deadlocked {
		t.Fatalf("expected deadlock but got %v", res)
	}
}

func TestInterpretSingleThreadedDeadlockOnGuard(t *testing.T) {
	s := MustParseRules(`
    member(X,[X1|Rest],R) :-
        X =\= X1 | member(X,Rest,R).
    member(X,[X1|_],R) :-
        X == X1 | R := true.
    member(_, [], R) :- R := false.`)
	i := NewSingleThreadedInterpreter(s)
	// this would work in Prolog, but not in FGHC (suspends on X)
	q, _ := i.MustParseProcesses("member(1, [X], R)")
	res, deadlocked := i.interpretSinglethreaded(q)
	if !deadlocked {
		t.Fatalf("expected deadlock but got %v", res)
	}
}

func TestInterpretSingleThreadedDeadlockOnPrimitive(t *testing.T) {
	s := MustParseRules(`test(X,Y) :- isplus(Y, X, 1).`)
	i := NewSingleThreadedInterpreter(s)
	// this would work in Prolog, but not in FGHC (suspends on X)
	q, _ := i.MustParseProcesses("test(X, Y)")
	res, deadlocked := i.interpretSinglethreaded(q)
	if !deadlocked {
		t.Fatalf("expected deadlock but got %v", res)
	}
}

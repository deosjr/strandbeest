package main

/*
From Strand book, page 42

interpreter()
    for each initial process P
        put_process(P)                              { put P in process pool }
    repeat
        P := get_process()                          { get a process from pool }
        if (is_predefined(P)) execute(P)            { predefined process }
            else reduce(P)                          { otherwise, do reduction }
    until(empty pool)

reduce(P)
    COMMIT := False                                 { initialize Flags }
    repeat
        R := pick_untried_rule(P,S)                 { get a rule from S }
        R1 := fresh_copy(R)                         { copy the rule to R1 }
        M := CMatch(P,R1)                           { execute match/guard }
        if (M=Theta) then                           { CMatch succeeds? }
            COMMIT := True                          { finished looking }
            spawn_body(R1,Theta)                    { add processes to pool }
        until (COMMIT) or (all_rules_tried(P))      { reduced or done }
        if (not COMMIT) then put_process(P)         { return process to pool }

where a process looks like functor(Arg1, Arg2...)
and CMatch takes a process and a rule, returning Theta if match succeeds given 
the set of assignments Theta, and the guard also succeeds. Otherwise suspend.
Predefined processes are builtin functions.
Vars can only occur once in head of a rule; guards are used to check equality.
Writing a rule head like functor(X,X,Y) instead of functor(X,X1,Y) :- X == X1 | ..
is allowed as syntactical sugar.

At each step, the interpreter nondeterministically selects a process from the pool
and a rule from the program. The manner they are chosen cannot be assumed!

Example program:

sum(L,Sum) :- sum1(L,0,Sum).    % initialize accumulator to 0

sum1([X|Xs],A,Sum) :-           % destructure list
    A1 is A + X,                % add head to accumulator
    sum1(Xs,A1,Sum).            % sum rest of list
sum1([],A,Sum) :-               % end of list encountered
    Sum := A.                   % return sum

Initial processes: sum([1|L],R), L := [2,3].
Result: R = 6
*/

import (
    "fmt"
)

func main() {
    // variables in rules are numbered by first occurence, starting at 0
    // actual vars will be assigned during copying of a matched rule with fresh vars
    s := []rule{
        {
            head: process{functor:"sum", args: []expression{
                variable(0), variable(1),
            }},
            body: []process{
                {functor:"sum1", args: []expression{
                    variable(0), number(0), variable(1),
                }},
            },
        },
        {
            head: process{functor:"sum1", args: []expression{
                list{head:variable(0), tail:variable(1)}, variable(2), variable(3),
            }},
            body: []process{
                // todo: complex expressions / infix operators that include processes ( is(A1, +(A, X)) )
                {functor:"isplus", args: []expression{
                    variable(4), variable(2), variable(0),
                }},
                {functor:"sum1", args: []expression{
                    variable(1), variable(4), variable(3),
                }},
            },
        },
        {
            head: process{functor:"sum1", args: []expression{
                emptylist, variable(0), variable(1),
            }},
            body: []process{
                {functor:":=", args: []expression{
                    variable(1), variable(0),
                }},
            },
        },
    }
    numWorkers := 10
    i := NewInterpreter(s, numWorkers)
    q := []process{
        {functor:"sum", args: []expression{
            list{head:number(1), tail:variable(0)}, variable(1),
        }},
        {functor:":=", args: []expression{
            variable(0), list{head:number(2), tail: list{head:number(3), tail:emptylist}},
        }},
    }
    //res := i.interpret(q)
    res := i.interpretSinglethreaded(q)
    r := walk(res, variable(1))
    fmt.Printf("R = %s\n", r.PrintExpression())
}

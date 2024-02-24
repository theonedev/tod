package main

type Command interface {
	Execute(args []string)
}

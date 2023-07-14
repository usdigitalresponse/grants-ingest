package main

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGuardPanic(t *testing.T) {
	t.Run("panic with error", func(t *testing.T) {
		err := errors.New("this is an error")
		res, caught := GuardPanic(func() (a string) { panic(err) })
		var zeroValue string
		assert.Equal(t, zeroValue, res)
		assert.ErrorIs(t, err, caught)
	})
	t.Run("panic with string", func(t *testing.T) {
		errString := "this is a string"
		res, err := GuardPanic(func() (a int) { panic(errString) })
		var zeroValue int
		assert.Equal(t, zeroValue, res)
		assert.EqualError(t, err, errString)
	})
	t.Run("panic with int64", func(t *testing.T) {
		res, err := GuardPanic(func() string { panic(int64(1234)) })
		var zeroValue string
		assert.Equal(t, zeroValue, res)
		assert.EqualError(t, err, fmt.Errorf("unknown panic: 1234 of type int64").Error())
	})
	t.Run("panic with float64", func(t *testing.T) {
		res, err := GuardPanic(func() *string { panic(12.34) })
		assert.Nil(t, res)
		assert.EqualError(t, err, fmt.Errorf("unknown panic: 12.34 of type float64").Error())
	})
	t.Run("no panic", func(t *testing.T) {
		res, err := GuardPanic(func() string { return "do not panic" })
		assert.NoError(t, err)
		assert.Equal(t, "do not panic", res)
	})
}

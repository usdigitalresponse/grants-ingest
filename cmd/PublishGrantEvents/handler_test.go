package main

import (
	"testing"

	goenv "github.com/Netflix/go-env"
	"github.com/go-kit/log"
	"github.com/stretchr/testify/require"
)

func setupLambdaEnvForTesting(t *testing.T) {
	t.Helper()

	// Suppress normal lambda log output
	logger = log.NewNopLogger()

	// Configure environment variables
	err := goenv.Unmarshal(goenv.EnvSet{
		
	}, &env)
	require.NoError(t, err, "Error configuring lambda environment for testing")
}
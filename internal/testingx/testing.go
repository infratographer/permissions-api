// Package testingx contains functions and data to facilitate testing using the testing package.
package testingx

import (
	"context"
	"testing"
)

// TestFunc represents a function that consumes a test input and returns a result.
type TestFunc[T, U any] func(context.Context, T) TestResult[U]

// TestResult represents the result of a test.
type TestResult[U any] struct {
	Success U
	Err     error
}

// TestCase represents a named test case, combining the input with a function for checking the observed result.
type TestCase[T, U any] struct {
	Name      string
	Input     T
	SetupFn   func(context.Context) context.Context
	CheckFn   func(context.Context, *testing.T, TestResult[U])
	CleanupFn func(context.Context)
}

// RunTests runs all provided test cases using the given test function.
func RunTests[T, U any](ctx context.Context, t *testing.T, cases []TestCase[T, U], testFn TestFunc[T, U]) {
	for _, testCase := range cases {
		testCase := testCase

		t.Run(testCase.Name, func(t *testing.T) {
			t.Parallel()

			// Ensure we're closed over ctx
			ctx := ctx

			if testCase.SetupFn != nil {
				ctx = testCase.SetupFn(ctx)
			}

			if testCase.CleanupFn != nil {
				t.Cleanup(func() {
					testCase.CleanupFn(ctx)
				})
			}

			result := testFn(ctx, testCase.Input)
			testCase.CheckFn(ctx, t, result)
		})
	}
}

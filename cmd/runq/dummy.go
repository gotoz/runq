// This file makes linter such as "go vet" happy.

package main

// validateProcessSpec is defined in the main package of runc.
func validateProcessSpec(spec interface{}) error {
	return nil
}

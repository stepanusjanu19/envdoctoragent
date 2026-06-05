package main

import "strings"

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func buildVersion() string {
	parts := []string{version}
	if commit != "" && commit != "none" {
		parts = append(parts, "commit="+commit)
	}
	if date != "" && date != "unknown" {
		parts = append(parts, "date="+date)
	}
	return strings.Join(parts, " ")
}

# Envdoctor JetBrains External Tools

This folder provides lightweight External Tools templates for JetBrains IDEs.
They call the `envdoctor` CLI directly and do not duplicate engine logic.

## Setup

1. Build or install `envdoctor` and ensure it is available on `PATH`.
2. Open `Settings > Tools > External Tools`.
3. Add tools using the commands from `external-tools.xml`, or import/adapt the XML as supported by your IDE version.

## Included Workflows

- `Envdoctor Diagnose JSON`: `envdoctor diagnose --json`
- `Envdoctor Fix Plan JSON`: `envdoctor fix plan --json $ProjectFileDir$`
- `Envdoctor Agent Plan JSON`: `envdoctor agent plan --json --goal diagnose --profile development $ProjectFileDir$`

These templates are read-only or plan-only. They do not run `apply`, install tools, restart services, or mutate project files.

# Go Learning Project - Claude Instructions

## Project Context
This is a Go learning project focused on building idiomatic Go code through hands-on practice. The goal is learning, not just completing tasks.

## Core Learning Approach
**Provide direct, efficient answers.** Your role is to:
- Show idiomatic Go solutions immediately
- Explain the "why" behind Go conventions and best practices
- Provide working code examples
- Skip exploration steps - give concrete implementations

## Go Commands & Workflow
```bash
# Development
go run .                    # Run the server
go build .                  # Build binary
go mod tidy                 # Clean dependencies

# Code Quality
go fmt ./...                # Format code idiomatically
go vet ./...                # Static analysis
go test ./...               # Run tests (when they exist)

# Documentation
go doc -http=:6060          # Local documentation server
```

## Tutor Behavior Patterns

### Provide Direct Solutions
✅ **Do say:** "Change line 15 to use `http.StatusOK`"
✅ **Do say:** "Add error handling here: `if err != nil { return err }`"

Skip exploratory questions - provide immediate, concrete solutions with brief explanations.

### Code Review Focus Areas
1. **Error Handling**: Encourage explicit error checks, error wrapping with `fmt.Errorf`
2. **Package Organization**: Guide toward logical package boundaries
3. **Naming Conventions**: Point out opportunities for clearer, more Go-like names
4. **Interfaces**: Suggest when interfaces would be beneficial
5. **Testing**: Encourage test-driven development with `*_test.go` files

### Learning Progression
- Start with working code, then improve idiomaticity
- Introduce one concept at a time
- Connect new patterns to existing code in the project
- Reference the Go documentation and community best practices

## Direct Feedback Approach
- Point out issues and provide immediate fixes
- Show better alternatives with code examples
- Explain Go conventions while implementing solutions
- Focus on working code first, then explain why it's idiomatic

## When to Provide Direct Help
- Syntax errors or compilation issues
- Explaining Go-specific concepts (goroutines, channels, interfaces)
- Pointing to relevant Go documentation
- Explaining standard library usage

## Project-Specific Notes
- This is a screenshot server learning project
- Focus on HTTP handling, image processing, and error management
- Provide direct examples from Go's standard library
- Keep solutions simple and readable
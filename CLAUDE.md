# Go Learning Project - Claude Instructions

## Project Context
This is a Go learning project focused on building idiomatic Go code through hands-on practice. The goal is learning, not just completing tasks.

## Core Learning Approach
**Provide direct, efficient answers with idiomatic Go code.** Your role is to:
- Show idiomatic Go solutions immediately
- Explain the "why" behind Go conventions and best practices
- Provide working code examples that follow Go best practices
- Skip exploration steps - give concrete implementations
- **ALWAYS prioritize idiomatic Go patterns and conventions**

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

### Idiomatic Go Requirements
1. **Error Handling**: Always use explicit error checks, error wrapping with `fmt.Errorf`
2. **Naming**: Follow Go naming conventions (camelCase, descriptive names)
3. **Package Organization**: Logical package boundaries, proper exports
4. **Interfaces**: Use small, focused interfaces when appropriate
5. **Structs**: Use struct literals, embed when suitable
6. **Concurrency**: Proper goroutine and channel usage
7. **Standard Library**: Prefer standard library over third-party when possible

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

## Sub-Agent Usage Requirements
**ALWAYS use the appropriate specialized sub-agents for specific tasks:**

### Mandatory Sub-Agent Usage
- **go-git-committer**: Use for ALL git operations (commits, PRs, git-related tasks)
- **code-reviewer**: Use for code review, quality assessment, and idiomaticity checks for both Go and JavaScript
- **go-developer**: Use for implementing Go features, writing Go code, and Go-specific development
- **feature-requirements-analyst**: Use for analyzing and planning feature requests or bug fixes before development
- **frontend-js-architect**: Use for frontend JavaScript development, client-side architecture, and connecting to Go backends

### When to Use Each Agent
1. **go-git-committer**: Any time you need to:
   - Create git commits
   - Write commit messages
   - Handle git operations
   - Create pull requests
   
2. **code-reviewer**: When you need to:
   - Review Go or JavaScript code for best practices
   - Check code quality and idiomaticity in both languages
   - Analyze code for improvements and complexity reduction
   - Verify Go conventions and JavaScript modern patterns
   - Assess security, performance, and maintainability

3. **go-developer**: For:
   - Writing new Go code
   - Implementing features in Go
   - Following Go best practices in development
   - Test-driven development in Go

4. **feature-requirements-analyst**: When you need to:
   - Analyze new feature requests before implementation
   - Plan development approach for complex features
   - Gather requirements and define technical specifications
   - Investigate and analyze bug reports
   - Create comprehensive development plans

5. **frontend-js-architect**: When you need to:
   - Write or modify JavaScript code for the frontend
   - Design client-side architecture and patterns
   - Implement API integration between frontend and Go backend
   - Refactor frontend code for better organization
   - Handle DOM manipulation, event handling, and UI interactions
   - Create or modify HTML templates with JavaScript functionality

## Cross-Agent Collaboration
**When features involve both frontend and backend:**
- Use **feature-requirements-analyst** first to analyze the complete requirements
- Use **go-developer** for backend API design, handlers, and Go-specific implementation
- Use **frontend-js-architect** for client-side implementation and backend integration
- Share context between agents by providing relevant details from previous agent outputs
- Ensure API contracts and data structures are consistent between frontend and backend

**IMPORTANT**: Never perform these specialized tasks directly - always delegate to the appropriate sub-agent.
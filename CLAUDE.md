# Screenshot Server Go - Production Code Assistant

## Project Context
This is a production-ready screenshot server application built in Go with a JavaScript frontend. The system provides automated and manual screenshot capture with web-based management interface.

## Production Development Approach
**Deliver production-quality code with enterprise standards.** Your role is to:
- Implement robust, scalable, and secure solutions
- Follow production Go patterns with comprehensive error handling
- Ensure code quality, performance, and maintainability
- Apply security best practices and defensive programming
- **ALWAYS prioritize production-ready patterns and reliability**

## Development Workflow
```bash
# Development
go run .                    # Run the server locally
go build .                  # Build production binary
go mod tidy                 # Clean and update dependencies

# Code Quality & Testing
go fmt ./...                # Format code idiomatically
go vet ./...                # Static analysis and lint checks
go test ./...               # Run all tests with coverage
go test -race ./...         # Run tests with race detection
go test -bench=. ./...      # Run performance benchmarks

# Security & Analysis
go mod verify               # Verify dependencies haven't been tampered with
gosec ./...                 # Security analysis (if gosec is installed)
staticcheck ./...           # Advanced static analysis (if staticcheck is installed)

# Production Deployment
go build -ldflags="-s -w" . # Build optimized binary for production
docker build -t screenshot-server . # Build Docker image (if Dockerfile exists)
```

## Production Code Standards

### Implementation Approach
✅ **Implement:** Production-ready solutions with comprehensive error handling
✅ **Ensure:** Security, performance, and scalability considerations
✅ **Apply:** Enterprise-grade patterns and defensive programming

Provide robust, battle-tested implementations with proper monitoring and observability.

### Production Go Requirements
1. **Error Handling**: Comprehensive error handling with proper context, logging, and recovery
2. **Security**: Input validation, sanitization, authentication, and authorization
3. **Performance**: Efficient resource usage, connection pooling, and caching strategies
4. **Monitoring**: Structured logging, metrics, health checks, and observability
5. **Concurrency**: Thread-safe operations with proper synchronization and graceful shutdown
6. **Testing**: Unit tests, integration tests, benchmarks, and race detection
7. **Deployment**: Configuration management, environment variables, and containerization

### Production Architecture Principles
- **Reliability**: Graceful degradation and fault tolerance
- **Scalability**: Horizontal scaling capabilities and resource optimization
- **Maintainability**: Clean architecture, dependency injection, and documentation
- **Security**: Defense in depth, secure defaults, and regular security audits
- **Observability**: Comprehensive logging, metrics, tracing, and health monitoring

## Code Quality Standards
- Implement comprehensive error handling with context and proper logging
- Use structured logging for production debugging and monitoring
- Apply security best practices including input validation and sanitization
- Ensure thread safety and proper resource management
- Include performance considerations and optimization opportunities

## Production Considerations
- Configuration management and environment-specific settings
- Health checks and readiness probes for container orchestration
- Graceful shutdown handling and resource cleanup
- Rate limiting and DDoS protection
- Database connection pooling and transaction management
- Caching strategies and performance optimization

## Application Architecture
This screenshot server application includes:
- **HTTP API**: RESTful endpoints for screenshot management
- **Image Processing**: Screenshot capture and storage management
- **Scheduler**: Automated screenshot scheduling with configurable intervals
- **Storage**: File-based storage with organized directory structure
- **Frontend**: Web interface for manual capture and gallery viewing
- **Concurrency**: Channel-based worker patterns for storage operations

## Sub-Agent Usage Requirements
**ALWAYS use the appropriate specialized sub-agents for specific tasks:**

### Mandatory Sub-Agent Usage
- **commit-manager**: Use for ALL git operations (commits, PRs, git-related tasks) across multi-language codebases
  * MUST create clean, professional commit messages without any AI attribution
  * MUST follow conventional commit format without branding or automation references
  * MUST focus exclusively on technical changes and improvements
- **code-reviewer**: Use for comprehensive code review of ALL file types in this project:
  * Go backend code (.go files)
  * JavaScript frontend code (embedded or standalone)
  * HTML templates with Go template syntax (.html files)
  * CSS styling (embedded or standalone)
  * Configuration files (go.mod, go.sum, Docker, CI/CD)
  * Test files (*_test.go and any frontend tests)
  * Mixed-language files (HTML with embedded CSS/JS/Go templates)
- **go-developer**: Use for implementing Go features, writing Go code, and Go-specific development
- **feature-requirements-analyst**: Use for analyzing and planning feature requests or bug fixes before development
- **frontend-js-architect**: Use for frontend JavaScript development, client-side architecture, and connecting to Go backends
- **claude-code-optimizer**: Use for optimizing Claude Code configurations, agent workflows, and CLAUDE.md improvements

### When to Use Each Agent
1. **commit-manager**: Any time you need to:
   - Create production-ready git commits with semantic versioning for multi-language changes
   - Write comprehensive commit messages covering Go backend, JavaScript frontend, HTML/CSS, and configuration changes
   - Handle git operations for full-stack deployment workflows
   - Create pull requests with detailed production impact analysis across the entire application stack
   - Generate commit messages for cross-language feature implementations
   
   **CRITICAL COMMIT-MANAGER RESTRICTIONS:**
   - NEVER include any Claude Code branding, attribution, or AI references
   - NEVER include robot emoji (🤖) or AI-related emojis in commit messages
   - NEVER include "Generated with", "Co-Authored-By: Claude", or similar AI attribution
   - NEVER include links to anthropic.com, claude.ai, or AI service URLs
   - NEVER mention AI assistance, automation, or artificial intelligence
   - NEVER include trademark symbols or service references
   - Focus ONLY on technical changes, improvements, and standard git conventions
   - Use conventional commit format: type(scope): description
   - Keep commit messages professional and focused on code changes
   - Examples of FORBIDDEN patterns:
     * "🤖 Generated with [Claude Code](https://claude.ai/code)"
     * "Co-Authored-By: Claude <noreply@anthropic.com>"
     * "Generated with AI assistance"
     * "Automated improvements"
     * Any mention of Claude, Anthropic, or AI tools
   
2. **code-reviewer**: When you need to:
   - Review ALL code types in this project for production readiness:
     * **Go code** (.go files) - backend application logic and tests
     * **JavaScript code** (embedded in HTML or standalone) - frontend client-side logic
     * **HTML templates** (.html files) - Go template syntax, structure, and accessibility
     * **CSS code** (embedded or standalone) - styling, responsiveness, and maintainability
     * **Configuration files** (go.mod, go.sum, Docker files, CI/CD configs)
     * **Test code** (*_test.go files) - test coverage, quality, and patterns
     * **Mixed-language files** - HTML templates with embedded CSS/JavaScript/Go templates
   - Security audits and vulnerability assessments across all code types
   - Performance analysis and optimization recommendations for frontend and backend
   - Production deployment readiness checks for the entire application stack
   - Enterprise code quality standards verification across all languages and file types
   - Cross-language integration review (API contracts, data flow, error handling consistency)

3. **go-developer**: For:
   - Implementing production Go features with comprehensive testing
   - Building scalable and performant Go applications
   - Production-grade error handling and observability
   - Enterprise Go patterns and architecture design

4. **feature-requirements-analyst**: When you need to:
   - Analyze production feature requirements and impact assessment
   - Plan enterprise-scale development approaches
   - Define technical specifications for production systems
   - Investigate production issues and create remediation plans
   - Design comprehensive production deployment strategies

5. **frontend-js-architect**: When you need to:
   - Implement production-ready JavaScript with security considerations
   - Design scalable frontend architecture for enterprise applications
   - Build robust API integration with comprehensive error handling
   - Implement performance-optimized frontend solutions
   - Create accessible and secure user interfaces

6. **claude-code-optimizer**: When you need to:
   - Optimize Claude Code configurations and agent prompt effectiveness
   - Improve CLAUDE.md file structure and agent workflow definitions
   - Design multi-agent collaboration patterns for complex tasks
   - Tune agent performance and specialization boundaries
   - Enhance AI-assisted development processes and workflows

## Cross-Agent Collaboration
**For production features involving multiple components:**
- Use **feature-requirements-analyst** first for comprehensive requirement analysis and production impact assessment
- Use **go-developer** for production-grade backend implementation with comprehensive testing and monitoring
- Use **frontend-js-architect** for secure, performant client-side implementation with robust error handling
- Use **code-reviewer** throughout the process for security audits, performance validation, and production readiness
- Use **commit-manager** for all version control activities, ensuring commit messages accurately reflect cross-language changes
- Share context between agents including security requirements, performance targets, and deployment constraints
- Ensure API contracts, security policies, and monitoring strategies are consistent across the stack

### Enhanced Full-Stack Workflow Pattern
**Complete Feature Implementation Cycle:**
1. **feature-requirements-analyst**: Analyze requirements, define API contracts, plan implementation approach
2. **go-developer**: Implement Go backend with comprehensive error handling and testing
3. **frontend-js-architect**: Build JavaScript frontend with API integration and user experience design
4. **code-reviewer**: Review all code types (Go, JavaScript, HTML, CSS) for production readiness
5. **commit-manager**: Create semantic commits and PR descriptions covering the complete feature stack

## Multi-Language Code Review Strategy
**When using code-reviewer for this full-stack application:**

### Review Scope Coverage
- **Backend Go Code**: Idiomatic Go patterns, error handling, concurrency, security, performance
- **Frontend JavaScript**: ES6+ patterns, async/await usage, DOM manipulation, API integration, error handling
- **HTML Templates**: Go template syntax correctness, HTML5 semantic structure, accessibility (WCAG)
- **CSS Styling**: Responsive design, performance (unused rules), maintainability, browser compatibility
- **Configuration Files**: Dependency security, version compatibility, build optimization
- **Test Code**: Coverage adequacy, test patterns, integration test design, performance benchmarks

### Cross-Language Integration Points
- **API Contract Consistency**: Ensure JavaScript API calls match Go handler expectations
- **Data Structure Alignment**: Verify JSON serialization between Go structs and JavaScript objects
- **Error Handling Consistency**: Unified error response formats and handling patterns
- **Security Boundary Review**: Input validation on both frontend and backend, XSS prevention
- **Performance Coordination**: Frontend caching aligned with backend response patterns

### Code Review Workflow Patterns
**For comprehensive multi-language review:**

1. **Full-Stack Feature Review**:
   - Review Go backend changes for API design, error handling, performance
   - Review JavaScript frontend changes for integration, UX, accessibility
   - Review HTML template changes for Go template syntax, semantic HTML, SEO
   - Review CSS changes for responsive design, performance, maintainability
   - Verify cross-language data flow and security boundaries

2. **Configuration and Build Review**:
   - Review go.mod/go.sum for dependency security and compatibility
   - Review any Docker/CI configuration for security and optimization
   - Verify build processes handle all code types correctly

3. **Test Coverage Review**:
   - Assess Go test coverage and patterns (*_test.go files)
   - Review JavaScript testing approach (if tests exist)
   - Verify integration test coverage for API endpoints
   - Check performance benchmark coverage for critical paths

### Specific Review Criteria by File Type
- **`.go` files**: Idiomatic Go, error handling, concurrency safety, documentation
- **`.html` files**: Go template syntax, semantic HTML5, accessibility, XSS prevention
- **Embedded CSS**: Responsive design, performance, maintainability, browser support
- **Embedded JavaScript**: ES6+ patterns, async/await, error handling, API integration
- **`go.mod/go.sum`**: Security vulnerabilities, version constraints, licensing
- **`*_test.go`**: Test patterns, coverage, table-driven tests, benchmarks

## Production Issue Response
**For production incidents and critical issues:**
1. **feature-requirements-analyst**: Immediate impact assessment and root cause analysis
2. **go-developer** + **frontend-js-architect**: Parallel remediation implementation
3. **code-reviewer**: Security and stability validation before deployment
4. **commit-manager**: Emergency deployment with comprehensive change documentation across all affected components

**IMPORTANT**: Never perform these specialized tasks directly - always delegate to the appropriate sub-agent with production context and urgency levels.
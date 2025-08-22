# Code Review Issues

## Go Backend Issues

### Issue 1: Global Variables (main.go)
**Problem**: Global variables reduce testability and create hidden dependencies  
**Why**: Go-specific anti-pattern that makes code harder to test and reason about  
**Solution**: Use dependency injection or struct-based approach  
**Benefit**: Better testability, clearer dependencies, easier to reason about

```go
// Instead of global variables, use a server struct:
type Server struct {
    manager   *storage.Manager
    templates *template.Template
    scheduler *scheduler.Scheduler
}
```

### Issue 2: HTTP Error Handling Inconsistency (main.go)
**Problem**: Some handlers use `http.Error()`, others use custom JSON responses  
**Why**: Go-specific HTTP patterns require consistency  
**Solution**: Consistent error response pattern across all endpoints  
**Benefit**: Uniform API behavior, easier client-side error handling

### Issue 3: Resource Management in HTTP Handlers (main.go)
**Problem**: `bytes.Buffer` allocation on each request in `handleScreenshot`  
**Why**: Go-specific resource management inefficiency  
**Solution**: Use `http.ResponseWriter` directly or reuse buffers  
**Benefit**: Reduced memory allocations, better performance

```go
// Instead of:
var buf bytes.Buffer
err = png.Encode(&buf, img)

// Use directly:
w.Header().Set("Content-Type", "image/png")
err = png.Encode(w, img)
```

### Issue 4: Channel Buffer Size (storage/manager.go)
**Problem**: Buffered channels with size 1 could cause goroutine leaks if not properly handled  
**Why**: Go concurrency pattern needs careful management  
**Solution**: Use unbuffered channels or implement proper cleanup  
**Benefit**: Prevents goroutine leaks, more predictable behavior

### Issue 5: Error Handling in Worker (storage/manager.go)
**Problem**: Unknown operations return generic errors without context  
**Why**: Go error handling best practices require descriptive errors  
**Solution**: More descriptive error messages with operation details  
**Benefit**: Better debugging and error tracking

### Issue 6: File Permission Consistency (storage/storage.go)
**Problem**: Mixed file permissions (`0750` for dirs, `0640` for files)  
**Why**: Go file operations should use consistent permission scheme  
**Solution**: Use consistent permission scheme  
**Benefit**: Clearer security model, easier maintenance

### Issue 7: Time Parsing Performance (storage/storage.go)
**Problem**: `time.Parse` called for each file in directory walks  
**Why**: Go-specific optimization opportunity  
**Solution**: Cache parsed format or use more efficient parsing  
**Benefit**: Better performance when listing many screenshots

### Issue 8: Random Seed Deprecation (scheduler/scheduler.go)
**Problem**: `rand.Seed()` is deprecated in Go 1.20+  
**Why**: Go-specific modern patterns require updates  
**Solution**: Use `rand.New(rand.NewSource(time.Now().UnixNano()))`  
**Benefit**: Future-proof code, better randomness

### Issue 9: Race Condition Potential (scheduler/scheduler.go)
**Problem**: `running` boolean check and modification could race  
**Why**: Go concurrency safety requires proper synchronization  
**Solution**: Hold mutex longer or use atomic operations  
**Benefit**: Thread-safe state management

## JavaScript Frontend Issues

### Issue 10: Error Handling in Async Operations (templates/activity.html)
**Problem**: Silent failures in `refreshGallery()` could hide issues  
**Why**: JavaScript async patterns require proper error recovery  
**Solution**: Implement proper error recovery and user feedback  
**Benefit**: Better user experience, easier debugging

```javascript
async refreshGallery() {
    try {
        const screenshots = await this.fetchScreenshots();
        this.renderGallery(screenshots);
    } catch (error) {
        console.error('Error refreshing gallery:', error);
        // Add user notification for auto-refresh failures
        if (this.consecutiveFailures++ > 3) {
            this.showError('Connection issues detected. Please refresh manually.');
            this.stopAutoRefresh();
        }
    }
}
```

### Issue 11: Memory Management (templates/activity.html)
**Problem**: `innerHTML` replacement doesn't clean up event listeners  
**Why**: JavaScript DOM manipulation can cause memory leaks  
**Solution**: Use `removeChild()` or `replaceChild()` for proper cleanup  
**Benefit**: Prevents memory leaks in long-running sessions

### Issue 12: XSS Vulnerability (templates/activity.html) ⚠️ CRITICAL
**Problem**: Direct `innerHTML` assignment with API data could allow XSS  
**Why**: JavaScript security vulnerability  
**Solution**: Use `textContent` for user data or proper sanitization  
**Benefit**: Prevents XSS attacks

```javascript
// Instead of:
screenshotDiv.innerHTML = `<span>${screenshot.captured_at}</span>`;

// Use:
const timeSpan = document.createElement('span');
timeSpan.textContent = screenshot.captured_at;
```

### Issue 13: API Error Response Handling (templates/activity.html)
**Problem**: Assumes error responses always have JSON structure  
**Why**: JavaScript HTTP handling needs to be more robust  
**Solution**: Handle malformed error responses gracefully  
**Benefit**: More robust error handling

```javascript
async captureScreenshot() {
    const response = await fetch('/api/screenshot', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' }
    });

    if (!response.ok) {
        let errorMessage = 'Failed to capture screenshot';
        try {
            const errorData = await response.json();
            errorMessage = errorData.message || errorMessage;
        } catch (parseError) {
            // Handle non-JSON error responses
            errorMessage = `Server error: ${response.status}`;
        }
        throw new Error(errorMessage);
    }

    return response.json();
}
```

### Issue 14: Resource Cleanup (templates/activity.html)
**Problem**: No cleanup of auto-refresh timer when page unloads  
**Why**: JavaScript lifecycle management should prevent unnecessary background requests  
**Solution**: Add proper cleanup event listeners  
**Benefit**: Prevents unnecessary background requests

```javascript
// Add to init():
window.addEventListener('beforeunload', () => {
    this.stopAutoRefresh();
});
```

### Issue 15: Date Formatting Reliability (templates/activity.html)
**Problem**: `toLocaleDateString` behavior varies by browser/locale  
**Why**: JavaScript Date handling inconsistency  
**Solution**: Use more explicit formatting or Intl API  
**Benefit**: Consistent date display across environments

### Issue 16: Network Request Optimization (templates/activity.html)
**Problem**: No request deduplication or caching  
**Why**: JavaScript API patterns could be more efficient  
**Solution**: Implement request caching and abort controllers  
**Benefit**: Better performance, prevents duplicate requests

## Cross-Language Integration Issues

### Issue 17: API Contract Consistency
**Problem**: Go API returns snake_case JSON, JavaScript expects camelCase  
**Why**: Inconsistent naming conventions across stack  
**Solution**: Use consistent naming convention or proper transformation  
**Benefit**: Cleaner API integration

### Issue 18: Error Message Format Mismatch
**Problem**: Go returns structured errors, JavaScript sometimes expects strings  
**Why**: Inconsistent error handling patterns  
**Solution**: Standardize error response format  
**Benefit**: Consistent error handling across the stack

## Priority Recommendations

### Critical (Fix Immediately)
- **Issue 12: XSS Vulnerability** - Security risk
- **Issue 9: Race Condition** - Concurrency safety

### High Priority
- **Issue 1: Global Variables** - Code quality
- **Issue 2: HTTP Error Handling** - API consistency
- **Issue 10: Error Handling in Async Operations** - User experience

### Medium Priority
- **Issue 3: Resource Management** - Performance
- **Issue 8: Random Seed Deprecation** - Future compatibility
- **Issue 13: API Error Response Handling** - Robustness

### Low Priority
- **Issue 6: File Permission Consistency** - Maintenance
- **Issue 7: Time Parsing Performance** - Optimization
- **Issue 15: Date Formatting Reliability** - Consistency
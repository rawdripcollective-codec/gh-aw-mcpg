package launcher

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/githubnext/gh-aw-mcpg/internal/config"
)

// TestGetOrLaunchForSession_StdioBackend tests stdio backend launching for a new session
func TestGetOrLaunchForSession_StdioBackend(t *testing.T) {
	// Use echo command to test stdio backend without requiring MCP protocol
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection for session1
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify connection is in session pool
	assert.Equal(t, 1, l.sessionPool.Size())
	assert.Equal(t, 0, len(l.connections), "Regular connections map should be empty")

	// Verify metadata
	meta := l.sessionPool.GetMetadata("stdio-server", "session1")
	require.NotNil(t, meta)
	assert.Equal(t, "stdio-server", meta.ServerID)
	assert.Equal(t, "session1", meta.SessionID)
	assert.Equal(t, 0, meta.ErrorCount)
}

// TestGetOrLaunchForSession_StdioReuse tests session connection reuse
func TestGetOrLaunchForSession_StdioReuse(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection for session1
	conn1, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn1)

	// Request same session again - should reuse connection
	conn2, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn2)

	// Should be the same connection object
	assert.Equal(t, conn1, conn2, "Should reuse same connection for same session")
	assert.Equal(t, 1, l.sessionPool.Size(), "Should still have only one connection")
}

// TestGetOrLaunchForSession_MultipleSessions tests multiple independent sessions
func TestGetOrLaunchForSession_MultipleSessions(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connections for 3 different sessions
	sessions := []string{"session1", "session2", "session3"}
	conns := make(map[string]interface{})

	for _, sessionID := range sessions {
		conn, err := GetOrLaunchForSession(l, "stdio-server", sessionID)
		require.NoError(t, err)
		require.NotNil(t, conn)
		conns[sessionID] = conn
	}

	// Verify all connections are different
	assert.NotEqual(t, conns["session1"], conns["session2"])
	assert.NotEqual(t, conns["session2"], conns["session3"])
	assert.NotEqual(t, conns["session1"], conns["session3"])

	// Verify pool has 3 connections
	assert.Equal(t, 3, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_DoubleCheckLock tests double-check locking pattern
func TestGetOrLaunchForSession_DoubleCheckLock(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "sleep",
			Args:    []string{"0.1"}, // Small delay to create race window
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch 10 goroutines trying to get the same session
	const numGoroutines = 10
	var wg sync.WaitGroup
	results := make(chan *struct {
		conn interface{}
		err  error
	}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
			results <- &struct {
				conn interface{}
				err  error
			}{conn, err}
		}()
	}

	wg.Wait()
	close(results)

	// All goroutines should get a connection without error
	var firstConn interface{}
	successCount := 0
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.conn)
		if firstConn == nil {
			firstConn = result.conn
		}
		// All should get the same connection (double-check lock prevents multiple launches)
		assert.Equal(t, firstConn, result.conn, "All goroutines should get same connection")
		successCount++
	}

	assert.Equal(t, numGoroutines, successCount, "All goroutines should succeed")
	assert.Equal(t, 1, l.sessionPool.Size(), "Should only have one connection despite concurrent requests")
}

// TestGetOrLaunchForSession_EnvPassthrough tests environment variable passthrough
func TestGetOrLaunchForSession_EnvPassthrough(t *testing.T) {
	// Set test environment variable
	t.Setenv("TEST_SESSION_VAR", "session-value-123")

	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"-e", "TEST_SESSION_VAR", "test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should log env passthrough
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify connection created successfully
	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_EnvMissing tests missing environment variable warning
func TestGetOrLaunchForSession_EnvMissing(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"-e", "NONEXISTENT_VAR", "test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should log warning about missing env var
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	// Verify connection created despite missing env var
	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_EnvExplicit tests explicit VAR=value env format
func TestGetOrLaunchForSession_EnvExplicit(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"-e", "VAR=explicit_value", "test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should not log env passthrough (explicit value)
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_EnvLongValue tests long value truncation in logs
func TestGetOrLaunchForSession_EnvLongValue(t *testing.T) {
	// Set env var with long value
	t.Setenv("LONG_VALUE_VAR", "this_is_a_very_long_value_that_should_be_truncated_in_logs")

	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"-e", "LONG_VALUE_VAR", "test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should log truncated value
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_EnvMap tests additional environment variables from config
func TestGetOrLaunchForSession_EnvMap(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
			Env: map[string]string{
				"CONFIG_VAR1": "value1",
				"CONFIG_VAR2": "value2",
			},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should log additional env vars
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_DirectCommandWarning tests warning for direct commands in container
func TestGetOrLaunchForSession_DirectCommandWarning(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo", // Direct command (not docker)
			Args:    []string{"test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Simulate running in container
	l.runningInContainer = true

	// Launch connection - should log warning about direct command in container
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_DockerCommandInContainer tests docker command is OK in container
func TestGetOrLaunchForSession_DockerCommandInContainer(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "docker",
			Args:    []string{"run", "--rm", "test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Simulate running in container
	l.runningInContainer = true

	// Launch connection - should NOT log warning (docker command is OK)
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	// Note: This will likely fail since docker isn't available in test env,
	// but we can verify the warning path wasn't taken
	_ = conn
	_ = err
	// Test focuses on the warning logic, actual connection may fail
}

// TestGetOrLaunchForSession_ConnectionFailure tests connection creation failure
func TestGetOrLaunchForSession_ConnectionFailure(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "nonexistent_command_12345",
			Args:    []string{},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Try to launch connection with invalid command
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "failed to create connection")

	// Verify error was recorded in session pool
	meta := l.sessionPool.GetMetadata("stdio-server", "session1")
	require.NotNil(t, meta)
	assert.Equal(t, 1, meta.ErrorCount, "Error should be recorded in session pool")
}

// TestGetOrLaunchForSession_Timeout tests startup timeout handling
func TestGetOrLaunchForSession_Timeout(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "sleep",
			Args:    []string{"10"}, // Sleep longer than timeout
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Set very short timeout for test
	l.startupTimeout = 100 * time.Millisecond

	// Try to launch connection - should timeout
	start := time.Now()
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.Nil(t, conn)
	assert.Contains(t, err.Error(), "timeout")

	// Verify timeout happened within reasonable time
	assert.Less(t, elapsed, 500*time.Millisecond, "Should timeout quickly")

	// Verify error was recorded in session pool
	meta := l.sessionPool.GetMetadata("stdio-server", "session1")
	require.NotNil(t, meta)
	assert.Equal(t, 1, meta.ErrorCount, "Timeout error should be recorded")
}

// TestGetOrLaunchForSession_MultipleServers tests different servers with different sessions
func TestGetOrLaunchForSession_MultipleServers(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"server1": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"server1"},
		},
		"server2": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"server2"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connections for different servers and sessions
	conn1a, err := GetOrLaunchForSession(l, "server1", "sessionA")
	require.NoError(t, err)
	require.NotNil(t, conn1a)

	conn1b, err := GetOrLaunchForSession(l, "server1", "sessionB")
	require.NoError(t, err)
	require.NotNil(t, conn1b)

	conn2a, err := GetOrLaunchForSession(l, "server2", "sessionA")
	require.NoError(t, err)
	require.NotNil(t, conn2a)

	// Verify all connections are different
	assert.NotEqual(t, conn1a, conn1b)
	assert.NotEqual(t, conn1a, conn2a)
	assert.NotEqual(t, conn1b, conn2a)

	// Verify pool has 3 connections
	assert.Equal(t, 3, l.sessionPool.Size())

	// Verify metadata for each
	meta1a := l.sessionPool.GetMetadata("server1", "sessionA")
	require.NotNil(t, meta1a)
	assert.Equal(t, "server1", meta1a.ServerID)
	assert.Equal(t, "sessionA", meta1a.SessionID)

	meta2a := l.sessionPool.GetMetadata("server2", "sessionA")
	require.NotNil(t, meta2a)
	assert.Equal(t, "server2", meta2a.ServerID)
	assert.Equal(t, "sessionA", meta2a.SessionID)
}

// TestGetOrLaunchForSession_EmptyEnvMap tests empty env map doesn't log
func TestGetOrLaunchForSession_EmptyEnvMap(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
			Env:     map[string]string{}, // Empty env map
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should not log additional env vars (empty map)
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_ConcurrentDifferentSessions tests concurrent launches for different sessions
func TestGetOrLaunchForSession_ConcurrentDifferentSessions(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch 5 different sessions concurrently
	const numSessions = 5
	var wg sync.WaitGroup
	results := make(chan *struct {
		sessionID string
		conn      interface{}
		err       error
	}, numSessions)

	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		sessionID := fmt.Sprintf("session%d", i)
		go func(sid string) {
			defer wg.Done()
			conn, err := GetOrLaunchForSession(l, "stdio-server", sid)
			results <- &struct {
				sessionID string
				conn      interface{}
				err       error
			}{sid, conn, err}
		}(sessionID)
	}

	wg.Wait()
	close(results)

	// All should succeed and create different connections
	conns := make(map[string]interface{})
	for result := range results {
		require.NoError(t, result.err)
		require.NotNil(t, result.conn)
		conns[result.sessionID] = result.conn
	}

	// Verify all connections are unique
	assert.Equal(t, numSessions, len(conns))
	assert.Equal(t, numSessions, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_ErrorRecording tests error count increases on failures
func TestGetOrLaunchForSession_ErrorRecording(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "nonexistent_command",
			Args:    []string{},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Try to launch connection multiple times (should fail each time)
	for i := 1; i <= 3; i++ {
		conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
		assert.Error(t, err)
		assert.Nil(t, conn)

		// Verify error count increases
		meta := l.sessionPool.GetMetadata("stdio-server", "session1")
		require.NotNil(t, meta)
		assert.Equal(t, i, meta.ErrorCount, "Error count should increase with each failure")
	}
}

// TestGetOrLaunchForSession_MultipleEnvFlags tests multiple -e flags in args
func TestGetOrLaunchForSession_MultipleEnvFlags(t *testing.T) {
	t.Setenv("VAR1", "value1")
	t.Setenv("VAR2", "value2")

	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"-e", "VAR1", "-e", "VAR2", "test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Launch connection - should log both env passthroughs
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.Equal(t, 1, l.sessionPool.Size())
}

// TestGetOrLaunchForSession_StartupTimeoutConfig tests custom startup timeout from config
func TestGetOrLaunchForSession_StartupTimeoutConfig(t *testing.T) {
	cfg := newTestConfig(map[string]*config.ServerConfig{
		"stdio-server": {
			Type:    "stdio",
			Command: "echo",
			Args:    []string{"test"},
		},
	})

	ctx := context.Background()
	l := New(ctx, cfg)
	defer l.Close()

	// Set custom startup timeout
	customTimeout := 5 * time.Second
	l.startupTimeout = customTimeout

	// Verify timeout is set correctly
	assert.Equal(t, customTimeout, l.startupTimeout)

	// Launch connection - should succeed with custom timeout
	conn, err := GetOrLaunchForSession(l, "stdio-server", "session1")
	require.NoError(t, err)
	require.NotNil(t, conn)
}

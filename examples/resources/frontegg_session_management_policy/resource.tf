resource "frontegg_session_management_policy" "example" {
  # End sessions after 30 minutes of inactivity.
  idle_session_timeout_enabled = true
  idle_session_timeout         = 1800 # 30 minutes

  # Force re-login after 7 days regardless of activity.
  force_relogin_enabled = true
  force_relogin_timeout = 604800 # 7 days

  # Allow at most 3 concurrent sessions per user.
  max_concurrent_sessions_enabled = true
  max_concurrent_sessions         = 3
}

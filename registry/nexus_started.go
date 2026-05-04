package registry

// NexusStartedRegistry is the caller-side Nexus start-phase registry. The
// Temporal SDK's started callback delivers `(token string, err error)` —
// token != "" for async, "" for sync. PHP's GetNexusOperationStarted{ID}
// listens here; the listener fires either when Push has already been called
// or as soon as Push happens, whichever comes first.
type NexusStartedRegistry = Registry[string]

// NexusStartedListener is the listener type for NexusStartedRegistry.
type NexusStartedListener = ListenerFunc[string]

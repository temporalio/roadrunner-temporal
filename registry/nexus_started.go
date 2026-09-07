package registry

// NexusStartedRegistry maps an ExecuteNexusOperation message ID to the SDK's
// started-callback result `(token, err)`. token != "" for async start, "" for sync.
type NexusStartedRegistry = Registry[string]

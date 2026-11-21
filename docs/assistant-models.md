# Assistant model selection and sign-in

This reference describes model selection and the installation-local ChatGPT sign-in path. Model credentials remain in the assistant runtime's private data directory and are not part of course synchronization.

## Selection

With no saved provider or model, the assistant uses the first usable catalog model. An explicit selection remains visible when its credentials expire or its model becomes unavailable; sending then fails with a settings hint instead of routing the question to another provider. The app serializes preference writes and waits for them before sending. A chat request may include `model_choice: {provider, model}`; the host rejects a mismatch with HTTP 409 before calling a model and updates an existing conversation to the current selection.

A retry status clears a stale error presentation. A final failed response still displays its error, and an incoming answer clears the preceding retry error.

## ChatGPT sign-in

In model settings, select “登录 ChatGPT” and complete authorization in the system browser. `POST /agent/openai-login` starts or joins one pending login and returns the SDK authorization URL; an already completed SDK flow may return `{status: "done"}`. `GET /agent/openai-login` returns `idle`, `waiting`, `done` or `error`. Only HTTPS authorization URLs at `auth.openai.com` are accepted. The SDK owns PKCE, callback validation, credential storage and refresh. A busy callback port (1455) prompts the user to finish the other sign-in and retry. If the agent restarts, polling stops immediately. The flow times out after five minutes; a failed exchange returns a generic network hint without token or provider-response contents.

Each installation has a private, random `oauth-device-id` file (mode 0600). After authorization, the host chooses an available OpenAI catalog model, preserving the current model ID if it is still offered. No model ID or user credential is embedded in the app. Each device signs in separately.

On macOS, the daemon passes the network service's explicit HTTP/HTTPS proxy to its Node assistant because Finder launches do not inherit a shell profile. Loopback addresses bypass that proxy. Android uses its own network connection; no USB bridge or remote credential copying is required by this feature.

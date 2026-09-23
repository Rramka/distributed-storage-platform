Protobuf contracts for control-plane RPCs, placement tickets, and receipts.

No `.proto` files on the solo track. Internal RPC is HTTP/JSON over TLS
(registration) / mTLS (heartbeats). gRPC and protobuf are deferred past M4
(see `docs/08-api.md` and `docs/05-node-agent.md`).

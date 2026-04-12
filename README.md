# mailtm (Go SDK)

A minimal, stdlib-only SDK for the [Mail.tm](https://mail.tm) API.

- Temp accounts, auth, read/manage messages, download sources/attachments, SSE realtime.
- Thread-safe client; functional options; retries with backoff and rate limiting.
- No external deps (pure `net/http`, `context`, `time`).

## Install

```bash
go get github.com/msuny-c/mailtm@latest
```

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/msuny-c/mailtm"
)

func main() {
	ctx := context.Background()

	cli, err := mailtm.New(
		mailtm.WithUserAgent("mailtm-go/1.0"),
	)
	if err != nil { log.Fatal(err) }

	doms, err := cli.ListDomains(ctx, 1)
	if err != nil { log.Fatal(err) }
	addr := fmt.Sprintf("tester@%s", doms.Member[0].Domain)

	acc, err := cli.CreateAccount(ctx, addr, "strong-password-123")
	if err != nil { log.Fatal(err) }

	tok, err := cli.Token(ctx, acc.Address, "strong-password-123")
	if err != nil { log.Fatal(err) }

	authed := cli.WithToken(tok.Token)

	// Wait up to 30s for the first message via polling
	msg, err := authed.WaitForFirstMessage(ctx, mailtm.WaitOptions{
		Timeout:      30 * time.Second,
		PollInterval: 2 * time.Second,
	})
	if err != nil { log.Fatal(err) }
	fmt.Println("Got message:", msg.Subject)

	// Download raw EML
	f, _ := os.Create("message.eml")
	defer f.Close()
	_ = authed.DownloadByURL(ctx, msg.DownloadURL, f)
}
```

## Highlights

- **Base URL**: `https://api.mail.tm` (HTTPS).  
- **Auth**: Bearer JWT (`POST /token`), except `/accounts` and `/domains`.
- **Rate limits**: 8 QPS per IP (SDK defaults to 8 rps token bucket).
- **Format**: JSON-LD (Hydra) with `hydra:member`, `hydra:totalItems`, `hydra:view`.
- **Realtime**: Mercure SSE hub at `https://mercure.mail.tm/.well-known/mercure`, topic `/accounts/{id}`.

See `godoc` for the full API surface.

## License

MIT

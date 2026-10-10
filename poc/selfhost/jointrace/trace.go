// Package jointrace emits fixed, non-secret lifecycle stages on stderr only.
// These are diagnostics, never completion receipts or authorization evidence.
package jointrace

import "os"

type Stage string

const (
	ProductStopping Stage = "product_stopping"
	ProductJoined   Stage = "product_joined"
	WatchStopping   Stage = "watch_stopping"
	WatchJoined     Stage = "watch_joined"
	WorkersJoined   Stage = "workers_joined"
	NodeClosing     Stage = "node_closing"
	NodeClosed      Stage = "node_closed"
	SignerClosing   Stage = "signer_closing"
	SignerClosed    Stage = "signer_closed"
)

func Emit(stage Stage) {
	switch stage {
	case ProductStopping, ProductJoined, WatchStopping, WatchJoined, WorkersJoined, NodeClosing, NodeClosed, SignerClosing, SignerClosed:
		_, _ = os.Stderr.WriteString("JOIN_STAGE " + string(stage) + "\n")
	}
}

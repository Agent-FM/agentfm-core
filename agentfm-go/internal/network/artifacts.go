package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"agentfm/internal/metrics"
	"agentfm/internal/obs"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/pterm/pterm"
)

func SendArtifacts(ctx context.Context, h host.Host, bossID peer.ID, zipFilePath string, taskID string) error {
	fmt.Println("📦 Opening secure artifact channel to Boss...")

	dialCtx, cancel := context.WithTimeout(ctx, StreamDialTimeout)
	defer cancel()

	stream, err := h.NewStream(dialCtx, bossID, ArtifactProtocol)
	if err != nil {
		return fmt.Errorf("failed to open artifact stream: %w", err)
	}

	success := false
	defer func() {
		if success {
			_ = stream.Close()
		} else {
			_ = stream.Reset()
		}
	}()

	if err := stream.SetWriteDeadline(time.Now().Add(ArtifactStreamTimeout)); err != nil {
		return fmt.Errorf("failed to set artifact write deadline: %w", err)
	}

	file, err := os.Open(zipFilePath)
	if err != nil {
		return fmt.Errorf("failed to open zip file: %w", err)
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat file: %w", err)
	}

	if err := binary.Write(stream, binary.LittleEndian, stat.Size()); err != nil {
		return fmt.Errorf("failed to write size header: %w", err)
	}

	taskIDBytes := []byte(taskID)
	if err := binary.Write(stream, binary.LittleEndian, uint8(len(taskIDBytes))); err != nil {
		return fmt.Errorf("failed to write task id length: %w", err)
	}

	if _, err := stream.Write(taskIDBytes); err != nil {
		return fmt.Errorf("failed to write task id: %w", err)
	}

	bytesWritten, err := io.Copy(stream, file)
	if err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonReset).Inc()
		return fmt.Errorf("failed during artifact stream: %w", err)
	}

	success = true
	metrics.ArtifactBytesSentTotal.Add(float64(bytesWritten))
	fmt.Printf("✅ Successfully sent %d bytes of artifacts over the darknet!\n", bytesWritten)
	return nil
}

type progressWriter struct {
	io.Writer
	pb *pterm.ProgressbarPrinter
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	n, err = pw.Writer.Write(p)
	if n > 0 && pw.pb != nil {
		pw.pb.Add(n)
	}
	return
}

func HandleArtifactStream(stream network.Stream) {
	success := false
	defer func() {
		if success {
			_ = stream.Close()
		} else {
			_ = stream.Reset()
		}
	}()

	if err := stream.SetReadDeadline(time.Now().Add(ArtifactStreamTimeout)); err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonDeadline).Inc()
		slog.Error("set artifact read deadline", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "artifacts"))
		return
	}

	fmt.Println("\n📥 [INCOMING] Artifact stream detected from Worker!")

	var fileSize int64
	err := binary.Read(stream, binary.LittleEndian, &fileSize)
	if err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonDecode).Inc()
		slog.Error("read artifact size header", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "artifacts"))
		return
	}
	// Cap the declared size up front. Without this a malicious worker can
	// stream until disk fills (the 30-min ArtifactStreamTimeout is not a
	// payload bound). Combined with the io.LimitReader below this is a
	// belt-and-braces defense against worker-declared size header lies.
	if fileSize <= 0 || fileSize > MaxArtifactBytes {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonDecode).Inc()
		slog.Warn("rejecting oversize artifact",
			slog.Int64("declared_size", fileSize),
			slog.Int64("max", MaxArtifactBytes),
			slog.String(obs.FieldProtocol, "artifacts"),
		)
		return
	}

	var idLen uint8
	err = binary.Read(stream, binary.LittleEndian, &idLen)
	if err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonDecode).Inc()
		slog.Error("read artifact task-id length", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "artifacts"))
		return
	}

	idBytes := make([]byte, idLen)
	_, err = io.ReadFull(stream, idBytes)
	if err != nil {
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonDecode).Inc()
		slog.Error("read artifact task-id", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "artifacts"))
		return
	}

	taskID := string(idBytes)
	safeTaskID := filepath.Base(filepath.Clean(taskID))
	if safeTaskID == "." || safeTaskID == "/" || safeTaskID == "" {
		safeTaskID = fmt.Sprintf("fallback_%d", time.Now().UnixNano())
	}

	if err := os.MkdirAll("./agentfm_artifacts", 0755); err != nil {
		slog.Error("create artifacts dir", slog.Any(obs.FieldErr, err))
		return
	}
	destPath := filepath.Join(".", "agentfm_artifacts", safeTaskID+".zip")

	outFile, err := os.Create(destPath)
	if err != nil {
		slog.Error("create local artifact file", slog.Any(obs.FieldErr, err), slog.String("dest", destPath))
		return
	}
	defer outFile.Close()

	progressTitle := safeTaskID
	if len(progressTitle) > 8 {
		progressTitle = progressTitle[:8]
	}
	pb, _ := pterm.DefaultProgressbar.
		WithTotal(int(fileSize)).
		WithTitle(fmt.Sprintf("Downloading %s.zip", progressTitle)).
		Start()

	pw := &progressWriter{
		Writer: outFile,
		pb:     pb,
	}

	// LimitReader bounds the actual bytes copied at the wire-declared size.
	// If the worker declared 1 MiB then tries to ship 50 GiB, io.Copy
	// returns at 1 MiB (and bytesRead < fileSize triggers the truncation
	// branch below). Belt-and-braces with the MaxArtifactBytes cap above.
	bytesRead, err := io.Copy(pw, io.LimitReader(stream, fileSize))
	if err != nil {
		pb.Stop()
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonReset).Inc()
		slog.Error("download artifacts", slog.Any(obs.FieldErr, err), slog.String(obs.FieldProtocol, "artifacts"))
		return
	}
	pb.Stop()
	if bytesRead != fileSize {
		// Truncation: the worker declared more than it shipped. Don't
		// surface the partial as a legitimate artifact.
		metrics.StreamErrorsTotal.WithLabelValues(metrics.ProtocolArtifacts, metrics.ReasonReset).Inc()
		slog.Warn("artifact truncated",
			slog.Int64("declared_size", fileSize),
			slog.Int64("actual", bytesRead),
			slog.String(obs.FieldProtocol, "artifacts"),
		)
		return
	}
	success = true
	pterm.Success.Printfln("🎉 Transfer Complete! Securely saved %d bytes to %s", bytesRead, destPath)
	fmt.Println()
	pterm.Println(pterm.LightWhite("👉 Press [ENTER] to continue to the feedback menu."))
}

package network

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

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
		return fmt.Errorf("failed during artifact stream: %w", err)
	}

	success = true
	fmt.Printf("✅ Successfully sent %d bytes of artifacts over the darknet!\n", bytesWritten)
	return nil
}

type progressWriter struct {
	io.Writer
	pb *pterm.ProgressbarPrinter
}

func (pw *progressWriter) Write(p []byte) (n int, err error) {
	time.Sleep(2 * time.Millisecond)
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
		pterm.Error.Printfln("Failed to set artifact read deadline: %v", err)
		return
	}

	fmt.Println("\n📥 [INCOMING] Artifact stream detected from Worker!")

	var fileSize int64
	err := binary.Read(stream, binary.LittleEndian, &fileSize)
	if err != nil {
		pterm.Error.Printfln("Failed to read file size header: %v", err)
		return
	}

	var idLen uint8
	err = binary.Read(stream, binary.LittleEndian, &idLen)
	if err != nil {
		pterm.Error.Printfln("Failed to read TaskID length: %v", err)
		return
	}

	idBytes := make([]byte, idLen)
	_, err = io.ReadFull(stream, idBytes)
	if err != nil {
		pterm.Error.Printfln("Failed to read TaskID: %v", err)
		return
	}

	taskID := string(idBytes)
	safeTaskID := filepath.Base(filepath.Clean(taskID))
	if safeTaskID == "." || safeTaskID == "/" || safeTaskID == "" {
		safeTaskID = fmt.Sprintf("fallback_%d", time.Now().UnixNano())
	}

	if err := os.MkdirAll("./agentfm_artifacts", 0755); err != nil {
		pterm.Error.Printfln("Failed to create artifacts dir: %v", err)
		return
	}
	destPath := filepath.Join(".", "agentfm_artifacts", safeTaskID+".zip")

	outFile, err := os.Create(destPath)
	if err != nil {
		pterm.Error.Printfln("Failed to create local file: %v", err)
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

	bytesRead, err := io.Copy(pw, stream)
	if err != nil {
		pb.Stop()
		pterm.Error.Printfln("Error downloading artifacts: %v", err)
		return
	}
	pb.Stop()
	success = true
	pterm.Success.Printfln("🎉 Transfer Complete! Securely saved %d bytes to %s", bytesRead, destPath)
	fmt.Println()
	pterm.Println(pterm.LightWhite("👉 Press [ENTER] to continue to the feedback menu."))
}

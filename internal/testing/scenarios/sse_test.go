package scenarios

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/kkollsga/enk-regnskap/internal/core"
	apptest "github.com/kkollsga/enk-regnskap/internal/testing"
)

// Fase: live oppdatering via SSE. En mutasjon (slik en MCP-agent ville gjørt)
// skal kringkastes til en tilkoblet nettleser.

func TestSSEBroadcastsOnMutation(t *testing.T) {
	h := apptest.Start(t)

	resp, err := http.Get(h.BaseURL + "/events")
	if err != nil {
		t.Fatalf("koble til /events: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("/events status = %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	// Les den innledende ": connected"-kommentaren slik at abonnementet er aktivt.
	if _, err := reader.ReadString('\n'); err != nil {
		t.Fatalf("les innledende SSE-linje: %v", err)
	}

	// Utfor en mutasjon (som en MCP-agent kunne gjørt).
	go func() {
		time.Sleep(50 * time.Millisecond)
		h.App.AddIncome(h.Context(), core.ActorMCP, core.IncomeInput{
			Date: "2025-01-10", Description: "SSE-test", Currency: "NOK",
			CountryCode: "NO", AmountOrig: 1000, Category: "tjenesteinntekt",
		})
	}()

	// Vent på en data-hendelse for "income", med timeout.
	done := make(chan string, 1)
	go func() {
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				done <- ""
				return
			}
			if strings.HasPrefix(line, "data:") && strings.Contains(line, "income") {
				done <- line
				return
			}
		}
	}()

	select {
	case line := <-done:
		if !strings.Contains(line, "income") {
			t.Errorf("forventet income-hendelse, fikk %q", line)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("ingen SSE-hendelse mottatt innen tidsfristen")
	}
}

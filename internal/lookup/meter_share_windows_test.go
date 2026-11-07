//go:build windows

package lookup

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestSummaryOpenerUsesOnlyAdvertisedChordAndBalancedReleases(t *testing.T) {
	for _, version := range []int{0, 2, 3} {
		inputs, err := summaryOpenInputs(version)
		if err != nil {
			t.Fatal(err)
		}
		keys := []uint16{0x87}
		if version == 2 {
			keys = []uint16{0x11, 0x10, 0x79}
		} else if version == 3 {
			keys = []uint16{0x11, 0x10, 0x78}
		}
		if len(inputs) != len(keys)*2 {
			t.Fatal(inputs)
		}
		for i, key := range keys {
			if inputs[i].Type != 1 || inputs[i].Keyboard.Key != key || inputs[i].Keyboard.Flags != 0 || inputs[len(inputs)-1-i].Keyboard.Key != key || inputs[len(inputs)-1-i].Keyboard.Flags != 2 {
				t.Fatal(inputs)
			}
		}
		// Every partial send releases only keys actually held, in reverse order,
		// so F10 is released before modifiers and can never become plain F10.
		for sent := 0; sent <= len(inputs); sent++ {
			held := map[uint16]bool{}
			for _, in := range inputs[:sent] {
				held[in.Keyboard.Key] = in.Keyboard.Flags == 0
			}
			var want []uint16
			for i := len(keys) - 1; i >= 0; i-- {
				if held[keys[i]] {
					want = append(want, keys[i])
				}
			}
			var got []uint16
			for _, in := range summaryInputCleanup(inputs, sent) {
				if in.Keyboard.Flags != 2 {
					t.Fatal(in)
				}
				got = append(got, in.Keyboard.Key)
			}
			if !slices.Equal(got, want) {
				t.Fatal(sent, got, want)
			}
		}
	}
	for _, version := range []int{-1, 1, 4, 99} {
		if _, err := summaryOpenInputs(version); err == nil {
			t.Fatal(version)
		}
	}
}

func TestSummaryOpenerCancellationAndPartialSendAlwaysReleaseAcceptedKeys(t *testing.T) {
	for _, fail := range []int{0, 1, 2, 3, 4, 5, 6} {
		ctx, cancel := context.WithCancel(context.Background())
		var batches [][]reloadInput
		call := 0
		err := sendSummaryOpen(ctx, 2, func(in []reloadInput) (int, error) {
			batches = append(batches, slices.Clone(in))
			call++
			if call == 1 {
				if fail < 3 {
					return fail, fmt.Errorf("partial down")
				}
				cancel()
				return len(in), nil
			}
			if call == 2 && fail < 6 {
				return fail - 3, fmt.Errorf("partial up")
			}
			return len(in), nil
		})
		cancel()
		if err == nil {
			t.Fatal("cancelled opener reported success", fail)
		}
		inputs, _ := summaryOpenInputs(2)
		want := summaryInputCleanup(inputs, fail)
		if len(want) > 0 && !slices.Equal(batches[len(batches)-1], want) {
			t.Fatal(fail, batches, want)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	calls := 0
	if sendSummaryOpen(ctx, 2, func(in []reloadInput) (int, error) { calls++; return len(in), nil }) == nil || calls != 0 {
		t.Fatal("cancelled opener sent input")
	}
}

func TestSummaryTextInputsNeverEmitActionKeysEvenWithoutReceiverFocus(t *testing.T) {
	for _, text := range []string{
		encodeSummaryDigits("[LamdaMeters] 1. Lamda 123K DPS[LM!nonce_1]"),
		"[LamdaMeters] 界 Я 🐟[LM!nonce_1]",
		"0123456789",
	} {
		inputs, err := summaryTextInputs(text)
		if err != nil {
			t.Fatal(err)
		}
		units := []uint16{}
		for i, input := range inputs {
			key := input.Keyboard
			// Missing edit-box focus must never expose a number, modifier,
			// Enter or any other bindable virtual key to the game. Unicode
			// packet key-up is also required after the final character, when
			// receipt handling deliberately clears the edit-box focus.
			wantFlags := uint32(4)
			if i%2 != 0 {
				wantFlags = 6
			}
			if input.Type != 1 || key.Key != 0 || key.Flags != wantFlags || key.Time != 0 || key.Extra != 0 {
				t.Fatalf("bindable or non-text event at %d: %+v", i, input)
			}
			if i%2 == 0 {
				units = append(units, key.Scan)
			} else if key.Scan != inputs[i-1].Keyboard.Scan {
				t.Fatal("packet release differs from its press")
			}
		}
		if got := string(utf16.Decode(units)); got != text {
			t.Fatalf("report text changed: %q", got)
		}
	}
}

func TestSummaryTextInputsRejectControlsInvalidUTF8AndOversizedTransfers(t *testing.T) {
	for _, text := range []string{"", "1\n", "1\r", "1\t", "1\x00", "1\x7f", "\xff", strings.Repeat("1", 13001)} {
		if _, err := summaryTextInputs(text); err == nil {
			t.Fatalf("invalid transfer allowed: %q", text[:min(20, len(text))])
		}
	}
	if _, err := summaryTextInputs(strings.Repeat("1", 13000)); err != nil {
		t.Fatal("bounded maximum transfer rejected", err)
	}
}

func TestSummaryFunctionInputsUseOnlyReservedUnmodifiedF13ThroughF22(t *testing.T) {
	text := "9012345678"
	inputs, err := summaryFunctionInputs(text)
	if err != nil || len(inputs) != 2*len(text) {
		t.Fatal(err, len(inputs))
	}
	for i, input := range inputs {
		key := input.Keyboard
		wantKey := uint16(0x7c + text[i/2] - '0')
		wantFlags := uint32(0)
		if i%2 != 0 {
			wantFlags = 2
		}
		if input.Type != 1 || key.Key != wantKey || key.Key < 0x7c || key.Key > 0x85 || key.Scan != 0 || key.Flags != wantFlags {
			t.Fatal(i, input)
		}
	}
	for _, text := range []string{"", "abc", "/reload", "1\n", strings.Repeat("1", 13001)} {
		if _, err := summaryFunctionInputs(text); err == nil {
			t.Fatal("invalid reserved-key transfer allowed")
		}
	}
}

func TestSummaryPartialInputReleasesOnlyAcceptedUnpairedPress(t *testing.T) {
	for _, makeInputs := range []func(string) ([]reloadInput, error){summaryTextInputs, summaryFunctionInputs} {
		inputs, err := makeInputs("123")
		if err != nil {
			t.Fatal(err)
		}
		for sent := 0; sent <= len(inputs); sent++ {
			cleanup := summaryInputCleanup(inputs, sent)
			if sent%2 == 0 {
				if len(cleanup) != 0 {
					t.Fatal("spurious key-up could invoke a click binding", sent, cleanup)
				}
			} else if len(cleanup) != 1 || cleanup[0] != inputs[sent] {
				t.Fatal("must release only the accepted final press", sent, cleanup)
			}
		}
	}
}

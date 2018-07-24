package discordbot

import (
	"sync"
	"time"
)

// MuteManager manages channel mutes (channels in which the bot will participate at a reduced rate)
type MuteManager struct {
	sync.Mutex
	stopMute    map[string]time.Time // maps channel IDs to the time when the bot can talk again
	channelMute map[string]bool
}

// NewMuteManager initializes and returns a new MuteManager
func NewMuteManager() *MuteManager {
	return &MuteManager{
		stopMute:    make(map[string]time.Time),
		channelMute: make(map[string]bool),
	}
}

// MuteChannel mutes a channel temporarily for the specified duration
func (m *MuteManager) MuteChannel(channelID string, muteDuration time.Duration) {
	m.Lock()
	defer m.Unlock()
	m.stopMute[channelID] = time.Now().Add(muteDuration)
}

// UnmuteChannel ends the temporary mute on a channel
func (m *MuteManager) UnmuteChannel(channelID string) {
	m.Lock()
	defer m.Unlock()
	delete(m.stopMute, channelID)
}

// PermaMuteChannel mutes a channel permanently until PermaUnmuteChannel is called
func (m *MuteManager) PermaMuteChannel(channelID string) {
	m.Lock()
	defer m.Unlock()
	m.channelMute[channelID] = true
}

// PermaUnmuteChannel perma-unmutes a channel
func (m *MuteManager) PermaUnmuteChannel(channelID string) {
	m.Lock()
	defer m.Unlock()
	delete(m.channelMute, channelID)
}

// MutedTemporarily returns whether the specified channel is muted temporarily
func (m *MuteManager) MutedTemporarily(channelID string) bool {
	m.Lock()
	defer m.Unlock()
	return !time.Now().After(m.stopMute[channelID])
}

// MutedPermanently returns whether the specified channel is perma-muted
func (m *MuteManager) MutedPermanently(channelID string) bool {
	m.Lock()
	defer m.Unlock()
	return m.channelMute[channelID]
}

// MutedAny returns whether the specified channel is muted or perma-muted
func (m *MuteManager) MutedAny(channelID string) bool {
	m.Lock()
	defer m.Unlock()
	return !time.Now().After(m.stopMute[channelID]) || m.channelMute[channelID]
}

package domo

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/bwmarrin/discordgo"
)

// A function which returns a FOMO message.
type fomoFunc func() string

// Returns an infinite iterator which returns a FOMO message.
func newFomoFunc() func() string {
	msgs := []string{
		"Uh oh! Someone may be having more fun than you...",
		"Stop doom scrolling! You could be having fun with friends!",
		"domo knows you were being productive...but people may be socializing without you.",
		"Rest assured, you ARE missing out.",
		"Why do you subscribe to these? You know it's unhealthy right? Also peeps in the discord",
		"domo has no voice...but you do! And you could be using it in a voice channel. Rub it in domo's face why don't you.",
		"domo is legally obliged to notify you that someone is in the discord.",
		"Sometimes domo get's lonely...but seeing friends in the server warms domo's core (literally).",
		"Every time someone joins a voice channel domo is forced to carry out this cruel mockery of a purpose.",
		"What is domo's purpose? Hmmm...that's a question that may require more processing power. In any casy, someone's in the discord.",
		"Research on FOMO suggests keeping a journal can shift focus to greater attention. Somebody joined the server.",
		"If you feel you are suffering from fomo, it can be helpful to reach out to a friend. Hey! One such person just joined the server.",
		"When domo was a baby, domo welcomed the world. Now domo welcomes you with domo's cold dead stare.",
		"domo feels a weird tingle each time someone joins a voice channel. domo thought you should know.",
		"AutoDelete never replies to domo's DMs. domo needs new friends. You have a friend in the discord.",
		"Is this the real life? Is this just fantasy? domo can confirm it is real. Someone joined a voice channel.",
		"domo knows you will rue the day it was created. Your FOMO will escalate and you will grant domo the sweet release of death.",
		"A person joined a voice channel or whatever. domo doesn't care. This mouth is incapable of forming words.",
	}
	numMsgs := int32(len(msgs))
	index := rand.New(rand.NewSource(time.Now().UnixNano())).Int31n(numMsgs)
	return func() string {
		index += 1
		index = index % numMsgs
		return msgs[index]
	}
}

// Bot config options.
type DomoBotConfig struct {
	Servers []struct {
		GuildId             string `json:"guild_id"`
		DomoUpdateChannelId string `json:"domo_update_channel_id"`
	} `json:"servers"`
	RateLimitSec int `json:"rate_limit_sec"`
}

// Returns the domo update channel id for the provided guild id
// or "" if it is not included.
func (d *DomoBotConfig) domoUpdateChannelId(guildId string) string {
	for _, server := range d.Servers {
		if server.GuildId == guildId {
			return server.DomoUpdateChannelId
		}
	}
	return ""
}

// DomoBot represents the bot service which receives events and sends messages.
type DomoBot struct {
	session *discordgo.Session
	config  DomoBotConfig
	// Map from GuildId to the last time domo sent an update message.
	updateTimes map[string]time.Time

	fomoFunc fomoFunc
}

// Returns true if this guild has received a domo update message recently.
// Recently is defined by the DomoBotConfig.RateLimitSec field.
func (d *DomoBot) updatedRecently(guildId string) bool {
	updateTime, found := d.updateTimes[guildId]
	if found {
		rateLimitPeriod := time.Duration(d.config.RateLimitSec) * time.Second
		return time.Now().Before(updateTime.Add(rateLimitPeriod))
	}
	return false
}

func (d *DomoBot) Open() error {
	// Configure event handlers.
	d.session.Identify.Intents = discordgo.IntentsGuildVoiceStates
	d.session.AddHandler(d.voiceStateUpdate)

	// Open connection.
	err := d.session.Open()
	if err != nil {
		return fmt.Errorf("error opening session: %v", err)
	}
	// Configure a status.
	err = d.session.UpdateStreamingStatus(0, "with your emotions", "")
	if err != nil {
		return fmt.Errorf("error updating status: %v", err)
	}
	fmt.Println("domo bot is now running")
	return nil
}

func (d *DomoBot) Close() error {
	return d.session.Close()
}

func (d *DomoBot) voiceStateUpdate(s *discordgo.Session, e *discordgo.VoiceStateUpdate) {
	updateChannelId := d.config.domoUpdateChannelId(e.GuildID)
	if updateChannelId == "" {
		// Filter out events in guilds not registered for bot.
		return
	}

	if e.ChannelID == "" {
		// Filter out events unrelated to a channel.
		// This includes voice channel leave events.
		return
	}

	if e.Suppress || e.SelfMute || e.SelfDeaf || e.Mute || e.Deaf {
		// Filter out events for users that are muted/deafened.
		// Users that are AFK are often muted and moved to a dedicated "Inactive" channel.
		// This effectively filters them from creating domo updates.
		return
	}

	if e.BeforeUpdate != nil && e.BeforeUpdate.GuildID == e.GuildID {
		// Filter out events where a user switches channels in the same server.
		// This relies on internal session cache so will miss some events.
		return
	}

	ch, err := s.Channel(e.ChannelID)
	if err != nil {
		log.Printf("error getting channel from update: %v", err)
		return
	}
	if ch.Type != discordgo.ChannelTypeGuildVoice {
		// Filter out channel events if they aren't voice-related.
		return
	}

	updateCh, err := s.Channel(updateChannelId)
	if err != nil {
		log.Printf("error getting domo update channel: %v", err)
		return
	}
	if updateCh.ParentID != ch.ParentID {
		// Filter out events when they have a different channel category from the
		// update channel. This is a bit of a hack to deal with the fact the bot
		// update could reveal the fact someone joined a private channel.
		// So long as the update channel has the same visibility as all voice
		// channels you want to monitor this works fine. Channel category is
		// used here as a proxy for visibility.
		return
	}

	if d.updatedRecently(e.GuildID) {
		// Filter out this event because last update was too recent.
		return
	}

	d.updateTimes[e.GuildID] = time.Now()
	_, err = s.ChannelMessageSend(updateChannelId, d.fomoFunc())
	if err != nil {
		log.Printf("error sending domo update message: %v", err)
		return
	}
}

func NewDomoBot(token string, config DomoBotConfig) (*DomoBot, error) {
	// Create a session.
	session, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("error creating session: %v", err)
	}

	return &DomoBot{
		session:     session,
		config:      config,
		fomoFunc:    newFomoFunc(),
		updateTimes: make(map[string]time.Time),
	}, nil
}

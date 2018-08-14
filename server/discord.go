package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/apcera/termtables"
	"github.com/bwmarrin/discordgo"
	"github.com/turnage/graw/reddit"
)

const (
	reaction   = "👍"
	nReactions = 3
	prefix     = ">"
)

var (
	dBot         *discordgo.Session
	pendingPosts = make(map[string]reddit.Post)
	dToken       = os.Getenv("DISCORD_TOKEN")
	dChannelID   = os.Getenv("DISCORD_CHANNEL_ID")
	dRoleMention = os.Getenv("DISCORD_ROLE_MENTION")

	newPostMsg         = dRoleMention + " :exclamation: New score post :exclamation: Should I upload it? React :+1: to vote yes. I'll upload if we reach " + strconv.Itoa(nReactions) + " reactions (including mine).\n**%s** (post by `/u/%s`) https://redd.it/%s"
	startAssignedMsg   = ":+1: Started job `%s`, assigned to `%s`."
	startBackloggedMsg = ":+1: Added job `%s` to backlog."
	startFailureMsg    = ":-1: Starting job `%s` failed, maybe try removing and adding a :+1:."
)

// StartDiscord starts waiting for posts to upload and starts the
func StartDiscord(posts chan reddit.Post) (chan bool, error) {
	if dToken == "" {
		return nil, errors.New("environment variable DISCORD_TOKEN is not set")
	}
	if dChannelID == "" {
		return nil, errors.New("environment variable DISCORD_CHANNEL_ID is not set")
	}
	if dRoleMention == "" {
		return nil, errors.New("environment variable DISCORD_ROLE_MENTION is not set")
	}

	var err error
	if dBot, err = discordgo.New("Bot " + dToken); err != nil {
		return nil, err
	}
	if _, err = dBot.Channel(dChannelID); err != nil {
		return nil, err
	}

	dBot.AddHandler(HandleReaction)
	dBot.AddHandler(HandleMessage)

	if err = dBot.Open(); err != nil {
		return nil, err
	}

	done := make(chan bool)
	go func() {
		log.Println("[discord] starting Discord bot")
		for {
			select {
			case <-done:
				return
			case p := <-posts:
				wg.Add(1)
				HandlePost(p)
				wg.Done()
			}
		}
	}()
	return done, nil
}

// handlePost receives a new Reddit post and prompts Discord users to vote on it.
func HandlePost(p reddit.Post) {
	msg, err := sendMsgf(newPostMsg, p.Title, p.Author, p.ID)
	if err != nil {
		return
	}
	pendingPosts[msg.ID] = p
	if err = dBot.MessageReactionAdd(dChannelID, msg.ID, reaction); err != nil {
		log.Println("[discord] couldn't add reaction:", err)
		// Don't return here, it's not a big deal.
	}
}

// handleReactions handles a new reaction being added on a message.
func HandleReaction(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
	if e.ChannelID != dChannelID || e.Emoji.Name != reaction || e.UserID == dBot.State.User.ID {
		return
	}

	found := false
	for msg := range pendingPosts {
		if msg == e.MessageID {
			found = true
		}
	}
	if !found {
		return
	}

	msg, err := dBot.ChannelMessage(dChannelID, e.MessageID)
	if err != nil {
		log.Println("[discord] couldn't get message:", err)
		return
	}

	for _, r := range msg.Reactions {
		if r.Emoji.Name == reaction && r.Count == nReactions {
			post := pendingPosts[e.MessageID]
			job, err := NewJob(post)
			if err != nil {
				log.Println("[discord] creating/assigning job failed:", err)
				sendMsgf(startFailureMsg, post.ID)
				return
			}
			if job.Status == statusBacklogged {
				sendMsgf(startBackloggedMsg, post.ID)
			} else {
				sendMsgf(startAssignedMsg, post.ID, job.WorkerID.String)
			}
			delete(pendingPosts, e.MessageID)
		}
	}
}

// HandleMessage handles an incoming command to the bot.
func HandleMessage(_ *discordgo.Session, e *discordgo.MessageCreate) {
	if e.ChannelID != dChannelID || !strings.HasPrefix(e.Message.Content, prefix) {
		return
	}

	switch e.Message.Content[len(prefix):] {
	case "list jobs":
		ListActive()
	case "list backlog":
		ListBacklog()
	}
}

// ListActive lists all active jobs.
func ListActive() {
	jobs, err := GetActiveJobs()
	if err != nil {
		sendMsg(":-1: Database error.")
		return
	}
	listJobs(jobs)
}

// ListBacklog lists all backlogged job.
func ListBacklog() {
	jobs, err := GetBacklog()
	if err != nil {
		sendMsg(":-1: Database error.")
		return
	}
	listJobs(jobs)
}

// listJobs formats jobs into a table and sends it to the Discord channel.
func listJobs(jobs []*Job) {
	if len(jobs) == 0 {
		sendMsg("No jobs.")
		return
	}
	table := termtables.CreateTable()
	table.AddHeaders("Job", "Worker", "Status", "Created", "Updated")
	for _, j := range jobs {
		var worker string
		if j.WorkerID.Valid {
			worker = j.WorkerID.String
		} else {
			worker = "none"
		}
		table.AddRow(j.ID, worker, statusStr[j.Status], time.Since(j.CreatedAt), time.Since(j.UpdatedAt))
	}
	sendMsgf("```\n%s\n```", table.Render())
}

// sendMsg sends a Discord message.
func sendMsg(text string) (*discordgo.Message, error) {
	msg, err := dBot.ChannelMessageSend(dChannelID, text)
	if err != nil {
		log.Println("[discord] couldn't send message:", err)
		return nil, err
	}
	log.Println("[discord] sent message:", msg.Content)
	return msg, nil
}

// sendMsgf sends a formatted Discord message.
func sendMsgf(text string, args ...interface{}) (*discordgo.Message, error) {
	return sendMsg(fmt.Sprintf(text, args...))
}

package matrixbot

import (
	"context"
	"fmt"

	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

type MatrixBot struct {
	Client *mautrix.Client
	RoomID id.RoomID
}

func NewMatrixBot(homeserverURL, username, password, roomID string) (*MatrixBot, error) {
	// Create a new Matrix client
	client, err := mautrix.NewClient(homeserverURL, "", "")
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Log in to the Matrix server
	loginReq := mautrix.ReqLogin{
		Type: mautrix.AuthTypePassword,
		Identifier: mautrix.UserIdentifier{
			Type: mautrix.IdentifierTypeUser,
			User: username,
		},
		Password: password,
	}
	loginResp, err := client.Login(context.Background(), &loginReq)
	if err != nil {
		return nil, fmt.Errorf("failed to log in: %w", err)
	}

	// Set the access token on the client
	client.SetCredentials(loginResp.UserID, loginResp.AccessToken)

	return &MatrixBot{
		Client: client,
		RoomID: id.RoomID(roomID),
	}, nil
}

func (bot *MatrixBot) SendMessage(htmlMessage string) error {
	content := event.MessageEventContent{
		MsgType:       event.MsgText,
		Format:        "org.matrix.custom.html",
		FormattedBody: htmlMessage,
	}

	_, err := bot.Client.SendMessageEvent(context.Background(), bot.RoomID, event.EventMessage, content)
	if err != nil {
		return fmt.Errorf("failed to send formatted message: %w", err)
	}

	return nil
}

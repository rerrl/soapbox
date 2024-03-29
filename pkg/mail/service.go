package mail

import (
	"fmt"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
)

type Service struct {
	client *sendgrid.Client
}

func NewMailService(client *sendgrid.Client) *Service {
	return &Service{client: client}
}

func (s *Service) SendPinEmail(recipient, pin string) error {
	m := mail.NewV3Mail()
	m.SetFrom(mail.NewEmail("GeniusCafe.iD", "Services@GeniusCafe.iD"))
	m.SetTemplateID("d-bdd1ab78539a4796a1b4516651d41f7a")

	p := mail.NewPersonalization()
	p.AddTos(mail.NewEmail("", recipient))
	p.SetDynamicTemplateData("pin", pin)

	m.AddPersonalizations(p)

	resp, err := s.client.Send(m)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to send email %v", resp.Body)
	}

	return nil
}

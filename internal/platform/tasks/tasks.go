// Package tasks hands events to Cloud Tasks, which calls them back into this
// same service.
//
// Cloud Tasks is the queue: it holds the work, retries it with backoff and
// gives up when told to. The outbox is the record of what happened; this is the
// handover between them (docs/design.md, section 2.5).
package tasks

import (
	"context"
	"encoding/json"
	"fmt"

	cloudtasks "cloud.google.com/go/cloudtasks/apiv2"
	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/aleogr/marketplace/internal/platform/outbox"
)

// Client hands events to Cloud Tasks queues.
type Client struct {
	tasks *cloudtasks.Client
	// project and location say where the queues are; url is the address Cloud
	// Tasks calls back; invoker is the account it signs the callback as.
	project  string
	location string
	url      string
	invoker  string
	audience string
}

// Settings is what the deployment tells this client.
type Settings struct {
	Project  string
	Location string
	// URL is the address of this service, which Cloud Tasks calls back.
	URL string
	// Invoker is the service account Cloud Tasks signs the callback as, and
	// which the endpoint expects (internal/platform/httpx.Tasks).
	Invoker  string
	Audience string
}

// New returns a client, or an error when the deployment has not been told
// where its queues are.
func New(ctx context.Context, settings Settings) (*Client, error) {
	for name, value := range map[string]string{
		"TASKS_PROJECT":  settings.Project,
		"TASKS_LOCATION": settings.Location,
		"SERVICE_URL":    settings.URL,
		"TASKS_INVOKER":  settings.Invoker,
	} {
		if value == "" {
			return nil, fmt.Errorf("%s is not set, and a queue cannot be reached without it", name)
		}
	}

	client, err := cloudtasks.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("cannot reach Cloud Tasks: %w", err)
	}

	audience := settings.Audience
	if audience == "" {
		audience = settings.URL
	}

	return &Client{
		tasks:    client,
		project:  settings.Project,
		location: settings.Location,
		url:      settings.URL,
		invoker:  settings.Invoker,
		audience: audience,
	}, nil
}

// Close releases the client.
func (c *Client) Close() error { return c.tasks.Close() }

// Enqueue hands one event to the queue of its class.
//
// The task is named after the event, and that name is the idempotency key:
// Cloud Tasks refuses a task whose name it has seen, so a dispatcher that
// queued an event and died before recording it queues nothing the second time.
func (c *Client) Enqueue(ctx context.Context, event outbox.Event) error {
	body, err := json.Marshal(map[string]any{"event": event})
	if err != nil {
		return fmt.Errorf("cannot write the task's body: %w", err)
	}

	queue := fmt.Sprintf("projects/%s/locations/%s/queues/%s", c.project, c.location, event.Queue)

	_, err = c.tasks.CreateTask(ctx, &cloudtaskspb.CreateTaskRequest{
		Parent: queue,
		Task: &cloudtaskspb.Task{
			Name: queue + "/tasks/" + event.ID,
			MessageType: &cloudtaskspb.Task_HttpRequest{
				HttpRequest: &cloudtaskspb.HttpRequest{
					Url:        c.url + tasksPath,
					HttpMethod: cloudtaskspb.HttpMethod_POST,
					Headers:    map[string]string{"Content-Type": "application/json"},
					Body:       body,
					AuthorizationHeader: &cloudtaskspb.HttpRequest_OidcToken{
						OidcToken: &cloudtaskspb.OidcToken{
							ServiceAccountEmail: c.invoker,
							Audience:            c.audience,
						},
					},
				},
			},
		},
	})

	// A task this dispatcher already queued. The event is on its way, so this
	// is success: the alternative is an event delivered twice for no reason.
	if status.Code(err) == codes.AlreadyExists {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cannot queue the %s event: %w", event.Kind, err)
	}
	return nil
}

// tasksPath is where the callback lands. It is spelled here rather than
// imported, because the HTTP package imports this one's sibling and a cycle
// would be the price of sharing one string.
const tasksPath = "/internal/tasks"

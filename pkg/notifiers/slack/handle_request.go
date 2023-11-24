package slacknotifier

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	"github.com/slack-go/slack"

	"github.com/aws/aws-lambda-go/events"
	"github.com/common-fate/common-fate/accesshandler/pkg/providerregistry"
	"github.com/common-fate/common-fate/accesshandler/pkg/providers"
	"github.com/common-fate/common-fate/pkg/access"
	"github.com/common-fate/common-fate/pkg/gevent"
	"github.com/common-fate/common-fate/pkg/identity"
	"github.com/common-fate/common-fate/pkg/notifiers"
	"github.com/common-fate/common-fate/pkg/rule"
	"github.com/common-fate/common-fate/pkg/storage"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/ddb"
	"github.com/pkg/errors"
	"go.uber.org/zap"
)

func (n *SlackNotifier) HandleRequestEvent(ctx context.Context, log *zap.SugaredLogger, event events.CloudWatchEvent) error {
	var requestEvent gevent.RequestEventPayload
	err := json.Unmarshal(event.Detail, &requestEvent)
	if err != nil {
		return err
	}
	request := requestEvent.Request
	requestedRuleQuery := storage.GetAccessRuleVersion{ID: request.Rule, VersionID: request.RuleVersion}
	_, err = n.DB.Query(ctx, &requestedRuleQuery)
	if err != nil {
		return errors.Wrap(err, "getting access rule")
	}
	requestedRule := *requestedRuleQuery.Result
	requestingUserQuery := storage.GetUser{ID: request.RequestedBy}
	_, err = n.DB.Query(ctx, &requestingUserQuery)
	if err != nil {
		return errors.Wrap(err, "getting requestor")
	}

	switch event.DetailType {
	case gevent.RequestCreatedType:
		// only send slack notification if access request requires approval.
		// if access request was automatically approved then no slack notification is sent.
		// this is done to reduce slack notification noise. More here: CF-831
		if !requestedRule.Approval.IsRequired() {
			return nil
		}

		// Message the requesting user.
		msg := fmt.Sprintf("Your request to access *%s* requires approval. We've notified the approvers and will let you know once your request has been reviewed.", requestedRule.Name)
		fallback := fmt.Sprintf("Your request to access %s requires approval.", requestedRule.Name)
		if n.directMessageClient != nil {
			_, err = SendMessage(ctx, n.directMessageClient.client, requestingUserQuery.Result.Email, msg, fallback, nil)
			if err != nil {
				log.Errorw("Failed to send direct message", "email", requestingUserQuery.Result.Email, "msg", msg, "error", err)
			}
		}

		// Notify approvers
		reviewURL, err := notifiers.ReviewURL(n.FrontendURL, request.ID)
		if err != nil {
			return errors.Wrap(err, "building review URL")
		}

		requestArguments, err := n.RenderRequestArguments(ctx, log, request, requestedRule)
		if err != nil {
			log.Errorw("failed to generate request arguments, skipping including them in the slack message", "error", err)
		}

		err = n.sendWebHooks(ctx, log, request, requestArguments, requestedRule, requestingUserQuery, reviewURL)

		// If we can send slack messages, send them.
		if n.directMessageClient == nil {
			return nil
		}

		// get the requestor's Slack user ID if it exists to render it nicely in the message to approvers.
		var slackUserID string
		requestor, err := n.directMessageClient.client.GetUserByEmailContext(ctx, requestingUserQuery.Result.Email)
		if err != nil {
			zap.S().Infow("couldn't get slack user from requestor - falling back to email address", "requestor.id", requestingUserQuery.Result.ID, zap.Error(err))
		}
		if requestor != nil {
			slackUserID = requestor.ID
		}
		//

		channel, group := requestedRule.ToSlackInfo()

		log.Infow("Tagged slack member", "group", group)

		reviewerSummary, reviewerMsg := BuildRequestReviewMessage(RequestMessageOpts{
			Request:          request,
			RequestArguments: requestArguments,
			Rule:             requestedRule,
			RequestorSlackID: slackUserID,
			RequestorEmail:   requestingUserQuery.Result.Email,
			ReviewURLs:       reviewURL,
			TaggedUser:       group,
			IsWebhook:        false,
		})
		// Notify slack group
		if channel != "" {

			err = n.sendMessageToChannel(ctx, reviewerMsg, reviewerSummary, channel)

			if err != nil {
				log.Infow("failed to send slack message to slack group", "channelId", channel, zap.Error(err))
			}
		} else {

			err = n.notifyAllReviewers(ctx, log, request, reviewerMsg, reviewerSummary, msg)

			if err != nil {
				return err
			}
		}
	case gevent.RequestApprovedType:
		msg := fmt.Sprintf(":white_check_mark: Your request to access *%s* has been approved.", requestedRule.Name)
		fallback := fmt.Sprintf("Your request to access %s has been approved.", requestedRule.Name)
		n.sendRequestDetailsMessage(ctx, log, request, requestedRule, *requestingUserQuery.Result, msg, fallback)
		n.SendUpdatesForRequest(ctx, log, request, requestEvent, requestedRule, requestingUserQuery.Result)
	case gevent.RequestCancelledType:
		n.SendUpdatesForRequest(ctx, log, request, requestEvent, requestedRule, requestingUserQuery.Result)
	case gevent.RequestDeclinedType:
		msg := fmt.Sprintf("Your request to access *%s* has been declined.", requestedRule.Name)
		fallback := fmt.Sprintf("Your request to access %s has been declined.", requestedRule.Name)
		n.SendDMWithLogOnError(ctx, log, request.RequestedBy, msg, fallback)
		n.SendUpdatesForRequest(ctx, log, request, requestEvent, requestedRule, requestingUserQuery.Result)
	}
	return nil
}

func (n *SlackNotifier) notifyAllReviewers(ctx context.Context, log *zap.SugaredLogger, request access.Request, reviewerMsg slack.Message, reviewerSummary string, msg string) error {
	reviewers := storage.ListRequestReviewers{RequestID: request.ID}
	_, err := n.DB.Query(ctx, &reviewers)
	if err != nil && err != ddb.ErrNoItems {
		return errors.Wrap(err, "getting reviewers")
	}

	log.Infow("messaging reviewers", "reviewers", reviewers)

	var wg sync.WaitGroup
	for _, usr := range reviewers.Result {
		if usr.ReviewerID == request.RequestedBy {
			log.Infow("skipping sending approval message to requestor", "user.id", usr)
			continue
		}
		wg.Add(1)
		go func(usr access.Reviewer) {
			defer wg.Done()
			approver := storage.GetUser{ID: usr.ReviewerID}
			_, err := n.DB.Query(ctx, &approver)
			if err != nil {
				log.Errorw("failed to fetch user by id while trying to send message in slack", "user.id", usr, zap.Error(err))
				return
			}
			ts, err := SendMessageBlocks(ctx, n.directMessageClient.client, approver.Result.Email, reviewerMsg, reviewerSummary)
			if err != nil {
				log.Errorw("failed to send request approval message", "user", usr, "msg", msg, zap.Error(err))
			}

			updatedUsr := usr
			updatedUsr.Notifications = access.Notifications{
				SlackMessageID: &ts,
			}
			log.Infow("updating reviewer with slack msg id", "updatedUsr.SlackMessageID", ts)

			err = n.DB.Put(ctx, &updatedUsr)

			if err != nil {
				log.Errorw("failed to update reviewer", "user", usr, zap.Error(err))
			}
		}(usr)
	}
	wg.Wait()
	return nil
}

func (n *SlackNotifier) sendWebHooks(
	ctx context.Context,
	log *zap.SugaredLogger,
	request access.Request,
	requestArguments []types.With,
	requestedRule rule.AccessRule,
	requestingUserQuery storage.GetUser,
	reviewURL notifiers.ReviewURLs) error {

	reviewerSummaryWh, reviewerMsgWh := BuildRequestReviewMessage(RequestMessageOpts{
		Request:          request,
		RequestArguments: requestArguments,
		Rule:             requestedRule,
		RequestorEmail:   requestingUserQuery.Result.Email,
		ReviewURLs:       reviewURL,
		IsWebhook:        true,
	})

	// send the review message to any configured webhook channels channels
	for _, webhook := range n.webhooks {
		err := webhook.SendWebhookMessage(ctx, reviewerMsgWh.Blocks, reviewerSummaryWh)
		if err != nil {
			log.Errorw("failed to send review message to incomingWebhook channel", "error", err)
		}
	}
	return nil
}

func (n *SlackNotifier) sendMessageToChannel(ctx context.Context, message slack.Message, summary string, channelName string) error {

	//Workaround: The slack Schedule Message API supports channel name
	// whereas the PostMessage API only supports sending to a channel ID.
	postAt := strconv.FormatInt(time.Now().Add(time.Second*10).Unix(), 10)
	zap.S().Infow("Scheduling message for", "time", postAt)
	_, _, err := n.directMessageClient.client.ScheduleMessageContext(
		ctx,
		channelName,
		postAt,
		slack.MsgOptionBlocks(message.Blocks.BlockSet...),
		slack.MsgOptionText(summary, false))

	//_, _, _, err := n.directMessageClient.client.SendMessageContext(
	//	ctx,
	//	channelName,
	//	slack.MsgOptionBlocks(message.Blocks.BlockSet...),
	//	slack.MsgOptionText(summary, false))

	if err != nil {
		zap.S().Infow("Received error", "err", err)
		return err
	}

	zap.S().Infow("Successfully sent slack message", channelName)

	return nil
}

// sendRequestDetailsMessage sends a message to the user who requested access with details about the request. Sent only on access create/approved
func (n *SlackNotifier) sendRequestDetailsMessage(ctx context.Context, log *zap.SugaredLogger, request access.Request, requestedRule rule.AccessRule, requestingUser identity.User, headingMsg string, summary string) {
	requestArguments, err := n.RenderRequestArguments(ctx, log, request, requestedRule)
	if err != nil {
		log.Errorw("failed to generate request arguments, skipping including them in the slack message", "error", err)
	}

	if n.directMessageClient != nil || len(n.webhooks) > 0 {
		if n.directMessageClient != nil {
			_, msg := BuildRequestDetailMessage(RequestDetailMessageOpts{
				Request:          request,
				RequestArguments: requestArguments,
				Rule:             requestedRule,
				HeadingMessage:   headingMsg,
			})

			_, err = SendMessageBlocks(ctx, n.directMessageClient.client, requestingUser.Email, msg, summary)

			if err != nil {
				log.Errorw("failed to send slack message", "user", requestingUser, zap.Error(err))
			}
		}

		for _, webhook := range n.webhooks {
			if !requestedRule.Approval.IsRequired() {
				headingMsg = fmt.Sprintf(":white_check_mark: %s's request to access *%s* has been automatically approved.\n", requestingUser.Email, requestedRule.Name)

				summary = fmt.Sprintf("%s's request to access %s has been automatically approved.", requestingUser.Email, requestedRule.Name)
			}
			_, msg := BuildRequestDetailMessage(RequestDetailMessageOpts{
				Request:          request,
				RequestArguments: requestArguments,
				Rule:             requestedRule,
				HeadingMessage:   headingMsg,
			})

			err = webhook.SendWebhookMessage(ctx, msg.Blocks, summary)
			if err != nil {
				log.Errorw("failed to send slack message to webhook channel", "error", err)
			}
		}
	}
}

func (n *SlackNotifier) SendUpdatesForRequest(ctx context.Context, log *zap.SugaredLogger, request access.Request, requestEvent gevent.RequestEventPayload, rule rule.AccessRule, requestingUser *identity.User) {
	// Loop over the request reviewers
	reviewers := storage.ListRequestReviewers{RequestID: request.ID}
	_, err := n.DB.Query(ctx, &reviewers)
	if err != nil && err != ddb.ErrNoItems {
		log.Errorw("failed to fetch reviewers for request", zap.Error(err))
		return
	}
	reqReviewer := storage.GetUser{ID: requestEvent.ReviewerID}
	_, err = n.DB.Query(ctx, &reqReviewer)
	if err != nil && request.Status != access.CANCELLED {
		log.Errorw("failed to fetch reviewer for request which wasn't cancelled", zap.Error(err))
		return
	}
	reviewURL, err := notifiers.ReviewURL(n.FrontendURL, request.ID)
	if err != nil {
		log.Errorw("building review URL", zap.Error(err))
		return
	}
	requestArguments, err := n.RenderRequestArguments(ctx, log, request, rule)
	if err != nil {
		log.Errorw("failed to generate request arguments, skipping including them in the slack message", "error", err)
	}
	log.Infow("messaging reviewers", "reviewers", reviewers.Result)
	if n.directMessageClient != nil {
		// get the requestor's Slack user ID if it exists to render it nicely in the message to approvers.
		var slackUserID string
		requestor, err := n.directMessageClient.client.GetUserByEmailContext(ctx, requestingUser.Email)
		if err != nil {
			// log this instead of returning
			log.Errorw("failed to get slack user id, defaulting to email", "user", requestingUser.Email, zap.Error(err))
		}
		if requestor != nil {
			slackUserID = requestor.ID
		}
		_, msg := BuildRequestReviewMessage(RequestMessageOpts{
			Request:          request,
			RequestArguments: requestArguments,
			Rule:             rule,
			RequestorSlackID: slackUserID,
			RequestorEmail:   requestingUser.Email,
			ReviewURLs:       reviewURL,
			WasReviewed:      true,
			RequestReviewer:  reqReviewer.Result,
			IsWebhook:        false,
		})
		for _, usr := range reviewers.Result {
			err = n.UpdateMessageBlockForReviewer(ctx, usr, msg)
			if err != nil {
				log.Errorw("failed to update slack message", "user", usr, zap.Error(err))
			}
		}
	}

	// log for testing purposes
	if len(n.webhooks) > 0 {
		log.Infow("webhooks found", "webhooks", n.webhooks)
	}
	// this does not include the slackUserID because we don't have access to look it up
	summary, msg := BuildRequestReviewMessage(RequestMessageOpts{
		Request:          request,
		RequestArguments: requestArguments,
		Rule:             rule,
		RequestorEmail:   requestingUser.Email,
		ReviewURLs:       reviewURL,
		WasReviewed:      true,
		RequestReviewer:  reqReviewer.Result,
		IsWebhook:        true,
	})
	for _, webhook := range n.webhooks {
		err = webhook.SendWebhookMessage(ctx, msg.Blocks, summary)
		if err != nil {
			log.Errorw("failed to send review message to incomingWebhook channel", "error", err)
		}
	}
}

// This method maps request arguments in a deprecated way.
// it should be replaced eventually with a cache lookup for the options available for the access rule
func (n *SlackNotifier) RenderRequestArguments(ctx context.Context, log *zap.SugaredLogger, request access.Request, rule rule.AccessRule) ([]types.With, error) {
	// Consider adding a fallback if the cache lookup fails
	pq := storage.ListCachedProviderOptions{
		ProviderID: rule.Target.ProviderID,
	}
	_, err := n.DB.Query(ctx, &pq)
	if err != nil && err != ddb.ErrNoItems {
		log.Errorw("failed to fetch provider options while trying to send message in slack", "provider.id", rule.Target.ProviderID, zap.Error(err))
	}
	var labelArr []types.With
	// Lookup the provider, ignore errors
	// if provider is not found, fallback to using the argument key as the title
	_, provider, _ := providerregistry.Registry().GetLatestByShortType(rule.Target.BuiltInProviderType)
	for k, v := range request.SelectedWith {
		with := types.With{
			Label: v.Label,
			Value: v.Value,
			Title: k,
		}
		// attempt to get the title for the argument from the provider arg schema
		if provider != nil {
			if s, ok := provider.Provider.(providers.ArgSchemarer); ok {
				t, ok := s.ArgSchema()[k]
				if ok {
					with.Title = t.Title
				}
			}
		}
		labelArr = append(labelArr, with)
	}

	for k, v := range rule.Target.With {
		// only include the with values if it does not have any groups selected,
		// if it does have groups selected, it means that it was a selectable field
		// so this check avoids duplicate/inaccurate values in the slack message
		if _, ok := rule.Target.WithArgumentGroupOptions[k]; !ok {
			with := types.With{
				Value: v,
				Title: k,
				Label: v,
			}
			// attempt to get the title for the argument from the provider arg schema
			if provider != nil {
				if s, ok := provider.Provider.(providers.ArgSchemarer); ok {
					t, ok := s.ArgSchema()[k]
					if ok {
						with.Title = t.Title
					}
				}
			}
			for _, ao := range pq.Result {
				// if a value is found, set it to true with a label
				if ao.Arg == k && ao.Value == v {
					with.Label = ao.Label
					break
				}
			}
			labelArr = append(labelArr, with)
		}
	}

	// now sort labelArr by Title
	sort.Slice(labelArr, func(i, j int) bool {
		return labelArr[i].Title < labelArr[j].Title
	})
	return labelArr, nil
}

package accesssvc

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/benbjohnson/clock"
	"github.com/common-fate/apikit/apio"
	"github.com/common-fate/common-fate/pkg/access"
	"github.com/common-fate/common-fate/pkg/identity"
	"github.com/common-fate/common-fate/pkg/rule"
	accessMocks "github.com/common-fate/common-fate/pkg/service/accesssvc/mocks"
	"github.com/common-fate/common-fate/pkg/storage"
	"github.com/common-fate/common-fate/pkg/types"
	"github.com/common-fate/ddb"
	"github.com/common-fate/ddb/ddbmock"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
)

func TestNewRequest(t *testing.T) {
	type createGrantResponse struct {
		request *access.Request
		err     error
	}
	type testcase struct {
		name                         string
		in                           CreateRequestsOpts
		rule                         *rule.AccessRule
		ruleErr                      error
		wantErr                      error
		want                         []CreateRequestResult
		withCreateGrantResponse      createGrantResponse
		withGetGroupResponse         *storage.GetGroup
		withRequestArgumentsResponse map[string]types.RequestArgument
		currentRequestsForGrant      []access.Request
	}

	clk := clock.NewMock()
	autoApproval := types.AUTOMATIC
	reviewed := types.REVIEWED
	testcases := []testcase{
		{
			name: "ok, no approvers so should auto approve",
			//just passing the group here, technically a user isnt an approver
			in: CreateRequestsOpts{User: identity.User{Groups: []string{"a"}}},
			rule: &rule.AccessRule{
				Groups: []string{"a"},
			},
			want: []CreateRequestResult{
				{Request: access.Request{
					ID:             "-",
					Status:         access.APPROVED,
					CreatedAt:      clk.Now(),
					UpdatedAt:      clk.Now(),
					Grant:          &access.Grant{},
					ApprovalMethod: &autoApproval,
					SelectedWith:   make(map[string]access.Option),
				}},
			},
			withCreateGrantResponse: createGrantResponse{
				request: &access.Request{
					ID:             "-",
					Status:         access.APPROVED,
					CreatedAt:      clk.Now(),
					UpdatedAt:      clk.Now(),
					Grant:          &access.Grant{},
					ApprovalMethod: &autoApproval,
					SelectedWith:   make(map[string]access.Option),
				},
			},
			withRequestArgumentsResponse: map[string]types.RequestArgument{},
			currentRequestsForGrant:      []access.Request{},
		},
		{
			name: "fails because requested duration is greater than max duration",
			in: CreateRequestsOpts{User: identity.User{Groups: []string{"a"}}, Create: CreateRequests{
				Timing: types.RequestTiming{
					DurationSeconds: 20,
				},
			}},
			rule: &rule.AccessRule{
				Groups: []string{"a"},
				TimeConstraints: types.TimeConstraints{
					MaxDurationSeconds: 10,
				},
			},
			wantErr: &apio.APIError{
				Err:    errors.New("request validation failed"),
				Status: http.StatusBadRequest,
				Fields: []apio.FieldError{
					{
						Field: "timing.durationSeconds",
						Error: fmt.Sprintf("durationSeconds: %d exceeds the maximum duration seconds: %d", 20, 10),
					},
				},
			},
			withRequestArgumentsResponse: map[string]types.RequestArgument{},
			currentRequestsForGrant:      []access.Request{},
		},
		{
			name: "user not in correct group",
			in:   CreateRequestsOpts{User: identity.User{Groups: []string{"a"}}},
			rule: &rule.AccessRule{
				Groups: []string{"b"},
			},
			wantErr:                 ErrNoMatchingGroup,
			currentRequestsForGrant: []access.Request{},
		},
		{
			name:                    "rule not found",
			in:                      CreateRequestsOpts{User: identity.User{Groups: []string{"a"}}},
			ruleErr:                 ddb.ErrNoItems,
			wantErr:                 ErrRuleNotFound,
			currentRequestsForGrant: []access.Request{},
		},
		{
			name: "with reviewers",
			in:   CreateRequestsOpts{User: identity.User{Groups: []string{"a"}}},
			rule: &rule.AccessRule{
				Groups: []string{"a"},
				Approval: rule.Approval{
					Users: []string{"b"},
				},
			},
			want: []CreateRequestResult{
				{Request: access.Request{
					ID:             "-",
					Status:         access.PENDING,
					CreatedAt:      clk.Now(),
					UpdatedAt:      clk.Now(),
					ApprovalMethod: &reviewed,
					SelectedWith:   make(map[string]access.Option),
				},
					Reviewers: []access.Reviewer{
						{
							ReviewerID: "b",
							Request: access.Request{
								ID:             "-",
								Status:         access.PENDING,
								CreatedAt:      clk.Now(),
								UpdatedAt:      clk.Now(),
								ApprovalMethod: &reviewed,
								SelectedWith:   make(map[string]access.Option),
							},
						},
					}},
			},
			withRequestArgumentsResponse: map[string]types.RequestArgument{},
			currentRequestsForGrant:      []access.Request{},
		},
		{
			name: "requestor is approver on access rule",
			in:   CreateRequestsOpts{User: identity.User{ID: "a", Groups: []string{"a"}}},
			rule: &rule.AccessRule{
				Groups: []string{"a"},
				Approval: rule.Approval{
					Users: []string{"a", "b"},
				},
			},
			// user 'a' should not be included as an approver of this request,
			// as they made the request.
			want: []CreateRequestResult{
				{Request: access.Request{
					ID:             "-",
					RequestedBy:    "a",
					Status:         access.PENDING,
					CreatedAt:      clk.Now(),
					UpdatedAt:      clk.Now(),
					ApprovalMethod: &reviewed,
					SelectedWith:   make(map[string]access.Option),
				},
					Reviewers: []access.Reviewer{
						{
							ReviewerID: "b",
							Request: access.Request{
								ID:             "-",
								RequestedBy:    "a",
								Status:         access.PENDING,
								CreatedAt:      clk.Now(),
								UpdatedAt:      clk.Now(),
								ApprovalMethod: &reviewed,
								SelectedWith:   make(map[string]access.Option),
							},
						},
					}},
			},
			withRequestArgumentsResponse: map[string]types.RequestArgument{},
			currentRequestsForGrant:      []access.Request{},
		},
		{
			name: "requestor is in approver group on access rule",
			in:   CreateRequestsOpts{User: identity.User{ID: "a", Groups: []string{"a"}}},
			rule: &rule.AccessRule{
				Groups: []string{"a"},
				Approval: rule.Approval{
					Groups: []string{"b"},
				},
			},
			withGetGroupResponse: &storage.GetGroup{
				Result: &identity.Group{
					ID:    "b",
					Users: []string{"c"},
				},
			},
			// user 'a' should not be included as an approver of this request,
			// as they made the request.
			want: []CreateRequestResult{
				{Request: access.Request{
					ID:             "-",
					RequestedBy:    "a",
					Status:         access.PENDING,
					CreatedAt:      clk.Now(),
					UpdatedAt:      clk.Now(),
					ApprovalMethod: &reviewed,
					SelectedWith:   make(map[string]access.Option),
				},
					Reviewers: []access.Reviewer{
						{
							ReviewerID: "c",
							Request: access.Request{
								ID:             "-",
								RequestedBy:    "a",
								Status:         access.PENDING,
								CreatedAt:      clk.Now(),
								UpdatedAt:      clk.Now(),
								ApprovalMethod: &reviewed,
								SelectedWith:   make(map[string]access.Option),
							},
						},
					}},
			},
			withRequestArgumentsResponse: map[string]types.RequestArgument{},
			currentRequestsForGrant:      []access.Request{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			db := ddbmock.New(t)
			db.MockQueryWithErr(&storage.GetAccessRuleCurrent{Result: tc.rule}, tc.ruleErr)
			db.MockQuery(tc.withGetGroupResponse)
			db.MockQuery(&storage.ListRequestReviewers{})
			db.MockQuery(&storage.ListRequestsForUserAndRequestend{Result: tc.currentRequestsForGrant})
			ctrl := gomock.NewController(t)

			defer ctrl.Finish()

			ctrl2 := gomock.NewController(t)
			ep := accessMocks.NewMockEventPutter(ctrl2)
			ep.EXPECT().Put(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()

			workflowMock := accessMocks.NewMockWorkflow(ctrl)
			if tc.withCreateGrantResponse.request != nil {
				workflowMock.EXPECT().Grant(gomock.Any(), gomock.Any(), gomock.Any()).Return(tc.withCreateGrantResponse.request.Grant, tc.withCreateGrantResponse.err).AnyTimes()
			}

			ca := accessMocks.NewMockCacheService(ctrl)
			ca.EXPECT().LoadCachedProviderArgOptions(gomock.Any(), gomock.Any(), gomock.Any()).Return(false, nil, nil, nil).AnyTimes()
			rs := accessMocks.NewMockAccessRuleService(ctrl)
			if tc.withRequestArgumentsResponse != nil {
				rs.EXPECT().RequestArguments(gomock.Any(), tc.rule.Target).Return(tc.withRequestArgumentsResponse, nil)
			}
			s := Service{
				Clock:       clk,
				DB:          db,
				EventPutter: ep,
				Cache:       ca,
				Rules:       rs,
				Workflow:    workflowMock,
			}
			got, err := s.CreateRequests(context.Background(), tc.in)
			var gotWithoutIDs []CreateRequestResult
			// ignore the autogenerated ID for testing.
			for _, res := range got {
				res.Request.ID = "-"
				for i := range res.Reviewers {
					res.Reviewers[i].Request.ID = "-"
				}
				gotWithoutIDs = append(gotWithoutIDs, res)
			}

			assert.Equal(t, tc.want, gotWithoutIDs)
			if tc.wantErr != nil {
				assert.EqualError(t, err, tc.wantErr.Error())
			}
		})
	}

}

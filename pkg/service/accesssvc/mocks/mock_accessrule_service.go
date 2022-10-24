// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/common-fate/granted-approvals/pkg/service/accesssvc (interfaces: AccessRuleService)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	rule "github.com/common-fate/granted-approvals/pkg/rule"
	types "github.com/common-fate/granted-approvals/pkg/types"
	gomock "github.com/golang/mock/gomock"
)

// MockAccessRuleService is a mock of AccessRuleService interface.
type MockAccessRuleService struct {
	ctrl     *gomock.Controller
	recorder *MockAccessRuleServiceMockRecorder
}

// MockAccessRuleServiceMockRecorder is the mock recorder for MockAccessRuleService.
type MockAccessRuleServiceMockRecorder struct {
	mock *MockAccessRuleService
}

// NewMockAccessRuleService creates a new mock instance.
func NewMockAccessRuleService(ctrl *gomock.Controller) *MockAccessRuleService {
	mock := &MockAccessRuleService{ctrl: ctrl}
	mock.recorder = &MockAccessRuleServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockAccessRuleService) EXPECT() *MockAccessRuleServiceMockRecorder {
	return m.recorder
}

// RequestArguments mocks base method.
func (m *MockAccessRuleService) RequestArguments(arg0 context.Context, arg1 rule.Target) (map[string]types.RequestArgument, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RequestArguments", arg0, arg1)
	ret0, _ := ret[0].(map[string]types.RequestArgument)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RequestArguments indicates an expected call of RequestArguments.
func (mr *MockAccessRuleServiceMockRecorder) RequestArguments(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RequestArguments", reflect.TypeOf((*MockAccessRuleService)(nil).RequestArguments), arg0, arg1)
}

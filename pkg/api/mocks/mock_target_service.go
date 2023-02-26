// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/common-fate/common-fate/pkg/api (interfaces: TargetService)

// Package mocks is a generated GoMock package.
package mocks

import (
	context "context"
	reflect "reflect"

	target "github.com/common-fate/common-fate/pkg/target"
	types "github.com/common-fate/common-fate/pkg/types"
	gomock "github.com/golang/mock/gomock"
)

// MockTargetService is a mock of TargetService interface.
type MockTargetService struct {
	ctrl     *gomock.Controller
	recorder *MockTargetServiceMockRecorder
}

// MockTargetServiceMockRecorder is the mock recorder for MockTargetService.
type MockTargetServiceMockRecorder struct {
	mock *MockTargetService
}

// NewMockTargetService creates a new mock instance.
func NewMockTargetService(ctrl *gomock.Controller) *MockTargetService {
	mock := &MockTargetService{ctrl: ctrl}
	mock.recorder = &MockTargetServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockTargetService) EXPECT() *MockTargetServiceMockRecorder {
	return m.recorder
}

// CreateGroup mocks base method.
func (m *MockTargetService) CreateGroup(arg0 context.Context, arg1 types.CreateTargetGroupRequest) (*target.Group, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateGroup", arg0, arg1)
	ret0, _ := ret[0].(*target.Group)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateGroup indicates an expected call of CreateGroup.
func (mr *MockTargetServiceMockRecorder) CreateGroup(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateGroup", reflect.TypeOf((*MockTargetService)(nil).CreateGroup), arg0, arg1)
}

// CreateRoute mocks base method.
func (m *MockTargetService) CreateRoute(arg0 context.Context, arg1 string, arg2 types.CreateTargetGroupLink) (*target.Route, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreateRoute", arg0, arg1, arg2)
	ret0, _ := ret[0].(*target.Route)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreateRoute indicates an expected call of CreateRoute.
func (mr *MockTargetServiceMockRecorder) CreateRoute(arg0, arg1, arg2 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreateRoute", reflect.TypeOf((*MockTargetService)(nil).CreateRoute), arg0, arg1, arg2)
}
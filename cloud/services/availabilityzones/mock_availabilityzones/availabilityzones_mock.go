/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by MockGen. DO NOT EDIT.
// Source: ../client.go

// Package mock_availabilityzones is a generated GoMock package.
package mock_availabilityzones

import (
	context "context"
	reflect "reflect"

	compute "github.com/Azure/azure-sdk-for-go/profiles/2019-03-01/compute/mgmt/compute"
	gomock "github.com/golang/mock/gomock"
)

// MockClient is a mock of Client interface.
type MockClient struct {
	ctrl     *gomock.Controller
	recorder *MockClientMockRecorder
}

// MockClientMockRecorder is the mock recorder for MockClient.
type MockClientMockRecorder struct {
	mock *MockClient
}

// NewMockClient creates a new mock instance.
func NewMockClient(ctrl *gomock.Controller) *MockClient {
	mock := &MockClient{ctrl: ctrl}
	mock.recorder = &MockClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockClient) EXPECT() *MockClientMockRecorder {
	return m.recorder
}

// ListComplete mocks base method.
func (m *MockClient) ListComplete(arg0 context.Context) (compute.ResourceSkusResultIterator, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListComplete", arg0)
	ret0, _ := ret[0].(compute.ResourceSkusResultIterator)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListComplete indicates an expected call of ListComplete.
func (mr *MockClientMockRecorder) ListComplete(arg0, arg1 interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListComplete", reflect.TypeOf((*MockClient)(nil).ListComplete), arg0, arg1)
}

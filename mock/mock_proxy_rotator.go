// Code generated by MockGen. DO NOT EDIT.
// Source: github.com/gosom/scrapemate (interfaces: ProxyRotator)
//
// Generated by this command:
//
//	mockgen -destination=mock/mock_proxy_rotator.go -package=mock . ProxyRotator
//

// Package mock is a generated GoMock package.
package mock

import (
	http "net/http"
	reflect "reflect"

	gomock "go.uber.org/mock/gomock"
)

// MockProxyRotator is a mock of ProxyRotator interface.
type MockProxyRotator struct {
	ctrl     *gomock.Controller
	recorder *MockProxyRotatorMockRecorder
}

// MockProxyRotatorMockRecorder is the mock recorder for MockProxyRotator.
type MockProxyRotatorMockRecorder struct {
	mock *MockProxyRotator
}

// NewMockProxyRotator creates a new mock instance.
func NewMockProxyRotator(ctrl *gomock.Controller) *MockProxyRotator {
	mock := &MockProxyRotator{ctrl: ctrl}
	mock.recorder = &MockProxyRotatorMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockProxyRotator) EXPECT() *MockProxyRotatorMockRecorder {
	return m.recorder
}

// GetCredentials mocks base method.
func (m *MockProxyRotator) GetCredentials() (string, string) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetCredentials")
	ret0, _ := ret[0].(string)
	ret1, _ := ret[1].(string)
	return ret0, ret1
}

// GetCredentials indicates an expected call of GetCredentials.
func (mr *MockProxyRotatorMockRecorder) GetCredentials() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetCredentials", reflect.TypeOf((*MockProxyRotator)(nil).GetCredentials))
}

// Next mocks base method.
func (m *MockProxyRotator) Next() string {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Next")
	ret0, _ := ret[0].(string)
	return ret0
}

// Next indicates an expected call of Next.
func (mr *MockProxyRotatorMockRecorder) Next() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Next", reflect.TypeOf((*MockProxyRotator)(nil).Next))
}

// RoundTrip mocks base method.
func (m *MockProxyRotator) RoundTrip(arg0 *http.Request) (*http.Response, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RoundTrip", arg0)
	ret0, _ := ret[0].(*http.Response)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// RoundTrip indicates an expected call of RoundTrip.
func (mr *MockProxyRotatorMockRecorder) RoundTrip(arg0 any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RoundTrip", reflect.TypeOf((*MockProxyRotator)(nil).RoundTrip), arg0)
}

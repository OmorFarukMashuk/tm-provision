package telmaxprovision

import (
	"errors"
)

func (request *ProvisionRequest) CheckValid() error {
	var err error
	if request.RequestType == "" {
		err = errors.New("RequestType is required")
	}
	return err
}

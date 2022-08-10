package database

import (
	"errors"
	"fmt"

	schema "github.com/SimifiniiCTO/simfiny-microservice-template/pkg/gen/proto/service_schema"
)

// validateAccountEmail asserts that an account email is not empty and valid
func (d *Db) validateAccountEmail(email string) (bool, error) {
	if email == "" {
		err := errors.New(fmt.Sprintf("invalid email. email: %s", email))
		d.Logger.Error(err.Error())
		return false, err
	}

	return true, nil
}

// validateAccountID asserts that an account ID is not zero and valid
func (d *Db) validateAccountID(acctID uint64) (bool, error) {
	if acctID == 0 {
		err := errors.New(fmt.Sprintf("invalid accID. acctID: %d", acctID))
		d.Logger.Error(err.Error())
		return false, err
	}

	return true, nil
}

// validateAccount asserts an account is valid and required params are present
func (d *Db) validateAccount(acct *schema.UserAccount) (bool, error) {
	if acct == nil {
		err := errors.New("invalid account object. account cannot be nil")
		d.Logger.Error(err.Error())
		return false, err
	}

	if valid, err := d.containsRequiredFields(acct); !valid {
		return false, err
	}

	return true, nil
}

// containsRequiredFields asserts an account object contains all necessary required fields
func (d *Db) containsRequiredFields(acct *schema.UserAccount) (bool, error) {
	if acct.Email == EMPTY || acct.Username == EMPTY || acct.Lastname == EMPTY || acct.Firstname == EMPTY {
		err := fmt.Errorf("invalid account object. username: %s, lastname: %s, firstname: %s, email: %s", acct.Username, acct.Lastname, acct.Firstname, acct.Email)
		d.Logger.Error(err.Error())
		return false, err
	}
	return true, nil
}

// validateAccountAndAccountID asserts and account and account ID are valid
func (d *Db) validateAccountAndAccountID(acct *schema.UserAccount, acctID uint64) (bool, error) {
	if acctValid, err := d.validateAccount(acct); !acctValid {
		return false, err
	}

	if acctIDValid, err := d.validateAccountID(acctID); !acctIDValid {
		return false, err
	}
	return true, nil
}

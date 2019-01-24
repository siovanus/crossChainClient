/*
 * Copyright (C) 2018 The ontology Authors
 * This file is part of The ontology library.
 *
 * The ontology is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The ontology is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The ontology.  If not, see <http://www.gnu.org/licenses/>.
 */

package common

import (
	"fmt"

	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/ontio/ontology/common/password"
)

func GetAccountByPassword(sdk *sdk.OntologySdk, path string) (*sdk.Account, bool) {
	wallet, err := sdk.OpenWallet(path)
	if err != nil {
		fmt.Println("open wallet error:", err)
		return nil, false
	}
	pwd, err := password.GetPassword()
	if err != nil {
		fmt.Println("getPassword error:", err)
		return nil, false
	}
	user, err := wallet.GetDefaultAccount(pwd)
	if err != nil {
		fmt.Println("getDefaultAccount error:", err)
		return nil, false
	}
	return user, true
}

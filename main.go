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
package main

import (
	"fmt"
	"github.com/ontio/crossChainClient/cmd"
	"github.com/ontio/crossChainClient/common"
	"github.com/ontio/crossChainClient/config"
	"github.com/ontio/crossChainClient/log"
	"github.com/ontio/crossChainClient/service"
	sdk "github.com/ontio/ontology-go-sdk"
	"github.com/urfave/cli"
	"os"
	"runtime"
)

func setupApp() *cli.App {
	app := cli.NewApp()
	app.Usage = "ONTDEX CLI"
	app.Action = startSync
	app.Copyright = "Copyright in 2018 The Ontology Authors"
	app.Flags = []cli.Flag{
		cmd.LogLevelFlag,
		cmd.ConfigPathFlag,
	}
	app.Commands = []cli.Command{}
	app.Before = func(context *cli.Context) error {
		runtime.GOMAXPROCS(runtime.NumCPU())
		return nil
	}
	return app
}

func main() {
	if err := setupApp().Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startSync(ctx *cli.Context) {
	logLevel := ctx.GlobalInt(cmd.GetFlagName(cmd.LogLevelFlag))
	log.InitLog(logLevel, log.PATH, log.Stdout)
	configPath := ctx.String(cmd.GetFlagName(cmd.ConfigPathFlag))
	err := config.DefConfig.Init(configPath)
	if err != nil {
		fmt.Println("DefConfig.Init error:", err)
		return
	}

	mainSdk := sdk.NewOntologySdk()
	mainSdk.NewRpcClient().SetAddress(config.DefConfig.MainJsonRpcAddress)
	sideSdk := sdk.NewOntologySdk()
	sideSdk.NewRpcClient().SetAddress(config.DefConfig.SideJsonRpcAddress)
	account, ok := common.GetAccountByPassword(mainSdk, config.DefConfig.WalletFile)
	if !ok {
		fmt.Println("common.GetAccountByPassword error")
		return
	}

	dexService := service.NewSyncService(account, mainSdk, sideSdk)
	dexService.Run()
}

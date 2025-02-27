package transfer_test

import (
	"errors"
	"math"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"

	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer"
	"github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	host "github.com/cosmos/ibc-go/v8/modules/core/24-host"
	ibcerrors "github.com/cosmos/ibc-go/v8/modules/core/errors"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"
	ibctesting "github.com/cosmos/ibc-go/v8/testing"
)

func (suite *TransferTestSuite) TestOnChanOpenInit() {
	var (
		channel      *channeltypes.Channel
		path         *ibctesting.Path
		chanCap      *capabilitytypes.Capability
		counterparty channeltypes.Counterparty
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success", func() {}, true,
		},
		{
			"empty version string", func() {
				channel.Version = ""
			}, true,
		},
		{
			"max channels reached", func() {
				path.EndpointA.ChannelID = channeltypes.FormatChannelIdentifier(math.MaxUint32 + 1)
			}, false,
		},
		{
			"invalid order - ORDERED", func() {
				channel.Ordering = channeltypes.ORDERED
			}, false,
		},
		{
			"invalid port ID", func() {
				path.EndpointA.ChannelConfig.PortID = ibctesting.MockPort
			}, false,
		},
		{
			"invalid version", func() {
				channel.Version = "version" //nolint:goconst
			}, false,
		},
		{
			"capability already claimed", func() {
				err := suite.chainA.GetSimApp().ScopedTransferKeeper.ClaimCapability(suite.chainA.GetContext(), chanCap, host.ChannelCapabilityPath(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID))
				suite.Require().NoError(err)
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset
			path = ibctesting.NewTransferPath(suite.chainA, suite.chainB)
			suite.coordinator.SetupConnections(path)
			path.EndpointA.ChannelID = ibctesting.FirstChannelID

			counterparty = channeltypes.NewCounterparty(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
			channel = &channeltypes.Channel{
				State:          channeltypes.INIT,
				Ordering:       channeltypes.UNORDERED,
				Counterparty:   counterparty,
				ConnectionHops: []string{path.EndpointA.ConnectionID},
				Version:        types.Version,
			}

			var err error
			chanCap, err = suite.chainA.App.GetScopedIBCKeeper().NewCapability(suite.chainA.GetContext(), host.ChannelCapabilityPath(ibctesting.TransferPort, path.EndpointA.ChannelID))
			suite.Require().NoError(err)

			tc.malleate() // explicitly change fields in channel and testChannel

			transferModule := transfer.NewIBCModule(suite.chainA.GetSimApp().TransferKeeper)
			version, err := transferModule.OnChanOpenInit(suite.chainA.GetContext(), channel.Ordering, channel.GetConnectionHops(),
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, chanCap, counterparty, channel.GetVersion(),
			)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(types.Version, version)
			} else {
				suite.Require().Error(err)
				suite.Require().Equal(version, "")
			}
		})
	}
}

func (suite *TransferTestSuite) TestOnChanOpenTry() {
	var (
		channel             *channeltypes.Channel
		chanCap             *capabilitytypes.Capability
		path                *ibctesting.Path
		counterparty        channeltypes.Counterparty
		counterpartyVersion string
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success", func() {}, true,
		},
		{
			"max channels reached", func() {
				path.EndpointA.ChannelID = channeltypes.FormatChannelIdentifier(math.MaxUint32 + 1)
			}, false,
		},
		{
			"capability already claimed", func() {
				err := suite.chainA.GetSimApp().ScopedTransferKeeper.ClaimCapability(suite.chainA.GetContext(), chanCap, host.ChannelCapabilityPath(path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID))
				suite.Require().NoError(err)
			}, false,
		},
		{
			"invalid order - ORDERED", func() {
				channel.Ordering = channeltypes.ORDERED
			}, false,
		},
		{
			"invalid port ID", func() {
				path.EndpointA.ChannelConfig.PortID = ibctesting.MockPort
			}, false,
		},
		{
			"invalid counterparty version", func() {
				counterpartyVersion = "version"
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path = ibctesting.NewTransferPath(suite.chainA, suite.chainB)
			suite.coordinator.SetupConnections(path)
			path.EndpointA.ChannelID = ibctesting.FirstChannelID

			counterparty = channeltypes.NewCounterparty(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
			channel = &channeltypes.Channel{
				State:          channeltypes.TRYOPEN,
				Ordering:       channeltypes.UNORDERED,
				Counterparty:   counterparty,
				ConnectionHops: []string{path.EndpointA.ConnectionID},
				Version:        types.Version,
			}
			counterpartyVersion = types.Version

			module, _, err := suite.chainA.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainA.GetContext(), ibctesting.TransferPort)
			suite.Require().NoError(err)

			chanCap, err = suite.chainA.App.GetScopedIBCKeeper().NewCapability(suite.chainA.GetContext(), host.ChannelCapabilityPath(ibctesting.TransferPort, path.EndpointA.ChannelID))
			suite.Require().NoError(err)

			cbs, ok := suite.chainA.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			tc.malleate() // explicitly change fields in channel and testChannel

			version, err := cbs.OnChanOpenTry(suite.chainA.GetContext(), channel.Ordering, channel.GetConnectionHops(),
				path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, chanCap, channel.Counterparty, counterpartyVersion,
			)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(types.Version, version)
			} else {
				suite.Require().Error(err)
				suite.Require().Equal("", version)
			}
		})
	}
}

func (suite *TransferTestSuite) TestOnChanOpenAck() {
	var counterpartyVersion string

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success", func() {}, true,
		},
		{
			"invalid counterparty version", func() {
				counterpartyVersion = "version"
			}, false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path := ibctesting.NewTransferPath(suite.chainA, suite.chainB)
			suite.coordinator.SetupConnections(path)
			path.EndpointA.ChannelID = ibctesting.FirstChannelID
			counterpartyVersion = types.Version

			module, _, err := suite.chainA.App.GetIBCKeeper().PortKeeper.LookupModuleByPort(suite.chainA.GetContext(), ibctesting.TransferPort)
			suite.Require().NoError(err)

			cbs, ok := suite.chainA.App.GetIBCKeeper().Router.GetRoute(module)
			suite.Require().True(ok)

			tc.malleate() // explicitly change fields in channel and testChannel

			err = cbs.OnChanOpenAck(suite.chainA.GetContext(), path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointA.Counterparty.ChannelID, counterpartyVersion)

			if tc.expPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

func (suite *TransferTestSuite) TestOnRecvPacket() {
	// This test suite mostly covers the top-level logic of the ibc module OnRecvPacket function
	// The core logic is covered in keeper OnRecvPacket
	var (
		packet             channeltypes.Packet
		expectedAttributes []sdk.Attribute
		path               *ibctesting.Path
	)
	testCases := []struct {
		name             string
		malleate         func()
		expAck           exported.Acknowledgement
		expEventErrorMsg string
	}{
		{
			"success", func() {}, channeltypes.NewResultAcknowledgement([]byte{byte(1)}), "",
		},
		{
			"failure: invalid packet data bytes",
			func() {
				packet.Data = []byte("invalid data")

				// Override expected attributes because this fails on unmarshaling packet data (so can't get the attributes)
				expectedAttributes = []sdk.Attribute{
					sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
					sdk.NewAttribute(sdk.AttributeKeySender, ""),
					sdk.NewAttribute(types.AttributeKeyReceiver, ""),
					sdk.NewAttribute(types.AttributeKeyDenom, ""),
					sdk.NewAttribute(types.AttributeKeyAmount, ""),
					sdk.NewAttribute(types.AttributeKeyMemo, ""),
					sdk.NewAttribute(types.AttributeKeyAckSuccess, "false"),
					sdk.NewAttribute(types.AttributeKeyAckError, "cannot unmarshal ICS-20 transfer packet data: invalid type"),
				}
			},
			channeltypes.NewErrorAcknowledgement(ibcerrors.ErrInvalidType),
			"cannot unmarshal ICS-20 transfer packet data: invalid type",
		},
		{
			"failure: receive disabled",
			func() {
				suite.chainB.GetSimApp().TransferKeeper.SetParams(suite.chainB.GetContext(), types.Params{ReceiveEnabled: false})
			},
			channeltypes.NewErrorAcknowledgement(types.ErrReceiveDisabled),
			"fungible token transfers to this chain are disabled",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path = ibctesting.NewTransferPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			coin := sdk.NewCoin(sdk.DefaultBondDenom, sdkmath.NewInt(100))
			packetData := types.NewFungibleTokenPacketData(
				coin.Denom,
				coin.Amount.String(),
				suite.chainA.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				"",
			)

			expectedAttributes = []sdk.Attribute{
				sdk.NewAttribute(sdk.AttributeKeyModule, types.ModuleName),
				sdk.NewAttribute(sdk.AttributeKeySender, packetData.Sender),
				sdk.NewAttribute(types.AttributeKeyReceiver, packetData.Receiver),
				sdk.NewAttribute(types.AttributeKeyDenom, packetData.Denom),
				sdk.NewAttribute(types.AttributeKeyAmount, packetData.Amount),
				sdk.NewAttribute(types.AttributeKeyMemo, packetData.Memo),
			}
			if tc.expAck == nil || tc.expAck.Success() {
				expectedAttributes = append(expectedAttributes, sdk.NewAttribute(types.AttributeKeyAckSuccess, "true"))
			} else {
				expectedAttributes = append(expectedAttributes,
					sdk.NewAttribute(types.AttributeKeyAckSuccess, "false"),
					sdk.NewAttribute(types.AttributeKeyAckError, tc.expEventErrorMsg),
				)
			}

			seq := uint64(1)
			timeout := suite.chainA.GetTimeoutHeight()
			packet = channeltypes.NewPacket(packetData.GetBytes(), seq, path.EndpointA.ChannelConfig.PortID, path.EndpointA.ChannelID, path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID, timeout, 0)

			ctx := suite.chainB.GetContext()
			cbs, ok := suite.chainB.App.GetIBCKeeper().PortKeeper.Router.GetRoute(ibctesting.TransferPort)
			suite.Require().True(ok)

			tc.malleate() // change fields in packet

			ack := cbs.OnRecvPacket(ctx, packet, suite.chainB.SenderAccount.GetAddress())

			suite.Require().Equal(tc.expAck, ack)

			expectedEvents := sdk.Events{
				sdk.NewEvent(
					types.EventTypePacket,
					expectedAttributes...,
				),
			}.ToABCIEvents()

			ibctesting.AssertEvents(&suite.Suite, expectedEvents, ctx.EventManager().Events().ToABCIEvents())
		})
	}
}

func (suite *TransferTestSuite) TestOnAcknowledgePacket() {
	var (
		path   *ibctesting.Path
		packet channeltypes.Packet
		ack    []byte
	)

	testCases := []struct {
		name      string
		malleate  func()
		expError  error
		expRefund bool
	}{
		{
			"success",
			func() {},
			nil,
			false,
		},
		{
			"success: refund coins",
			func() {
				ack = channeltypes.NewErrorAcknowledgement(types.ErrInvalidAmount).Acknowledgement()
			},
			nil,
			true,
		},
		{
			"cannot refund ack on non-existent channel",
			func() {
				ack = channeltypes.NewErrorAcknowledgement(types.ErrInvalidAmount).Acknowledgement()

				packet.SourceChannel = "channel-100"
			},
			errors.New("unable to unescrow tokens"),
			false,
		},
		{
			"invalid packet data",
			func() {
				packet.Data = []byte("invalid data")
			},
			ibcerrors.ErrUnknownRequest,
			false,
		},
		{
			"invalid acknowledgement",
			func() {
				ack = []byte("invalid ack")
			},
			ibcerrors.ErrUnknownRequest,
			false,
		},
		{
			"cannot refund already acknowledged packet",
			func() {
				ack = channeltypes.NewErrorAcknowledgement(ibcerrors.ErrInsufficientFunds).Acknowledgement()

				cbs, ok := suite.chainA.App.GetIBCKeeper().PortKeeper.Router.GetRoute(ibctesting.TransferPort)
				suite.Require().True(ok)

				suite.Require().NoError(cbs.OnAcknowledgementPacket(suite.chainA.GetContext(), packet, ack, suite.chainA.SenderAccount.GetAddress()))
			},
			errors.New("unable to unescrow tokens"),
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path = ibctesting.NewTransferPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			timeoutHeight := suite.chainA.GetTimeoutHeight()
			msg := types.NewMsgTransfer(
				path.EndpointA.ChannelConfig.PortID,
				path.EndpointA.ChannelID,
				ibctesting.TestCoin,
				suite.chainA.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				timeoutHeight,
				0,
				"",
			)
			res, err := suite.chainA.SendMsgs(msg)
			suite.Require().NoError(err) // message committed

			packet, err = ibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			cbs, ok := suite.chainA.App.GetIBCKeeper().PortKeeper.Router.GetRoute(ibctesting.TransferPort)
			suite.Require().True(ok)

			ack = channeltypes.NewResultAcknowledgement([]byte{byte(1)}).Acknowledgement()

			tc.malleate() // change fields in packet

			err = cbs.OnAcknowledgementPacket(suite.chainA.GetContext(), packet, ack, suite.chainA.SenderAccount.GetAddress())

			if tc.expError == nil {
				suite.Require().NoError(err)

				if tc.expRefund {
					escrowAddress := types.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())
					escrowBalanceAfter := suite.chainA.GetSimApp().BankKeeper.GetBalance(suite.chainA.GetContext(), escrowAddress, sdk.DefaultBondDenom)
					suite.Require().Equal(sdkmath.NewInt(0), escrowBalanceAfter.Amount)
				}
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expError.Error())
			}
		})
	}
}

func (suite *TransferTestSuite) TestOnTimeoutPacket() {
	var path *ibctesting.Path
	var packet channeltypes.Packet

	testCases := []struct {
		name           string
		coinsToSendToB sdk.Coin
		malleate       func()
		expError       error
	}{
		{
			"success",
			ibctesting.TestCoin,
			func() {},
			nil,
		},
		{
			"non-existent channel",
			ibctesting.TestCoin,
			func() {
				packet.SourceChannel = "channel-100"
			},
			errors.New("unable to unescrow tokens"),
		},
		{
			"invalid packet data",
			ibctesting.TestCoin,
			func() {
				packet.Data = []byte("invalid data")
			},
			ibcerrors.ErrUnknownRequest,
		},
		{
			"already timed-out packet",
			ibctesting.TestCoin,
			func() {
				cbs, ok := suite.chainA.App.GetIBCKeeper().PortKeeper.Router.GetRoute(ibctesting.TransferPort)
				suite.Require().True(ok)

				suite.Require().NoError(cbs.OnTimeoutPacket(suite.chainA.GetContext(), packet, suite.chainA.SenderAccount.GetAddress()))
			},
			errors.New("unable to unescrow tokens"),
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			path = ibctesting.NewTransferPath(suite.chainA, suite.chainB)
			suite.coordinator.Setup(path)

			timeoutHeight := suite.chainA.GetTimeoutHeight()
			msg := types.NewMsgTransfer(
				path.EndpointA.ChannelConfig.PortID,
				path.EndpointA.ChannelID,
				tc.coinsToSendToB,
				suite.chainA.SenderAccount.GetAddress().String(),
				suite.chainB.SenderAccount.GetAddress().String(),
				timeoutHeight,
				0,
				"",
			)
			res, err := suite.chainA.SendMsgs(msg)
			suite.Require().NoError(err) // message committed

			packet, err = ibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			cbs, ok := suite.chainA.App.GetIBCKeeper().PortKeeper.Router.GetRoute(ibctesting.TransferPort)
			suite.Require().True(ok)

			tc.malleate() // change fields in packet

			err = cbs.OnTimeoutPacket(suite.chainA.GetContext(), packet, suite.chainA.SenderAccount.GetAddress())

			if tc.expError == nil {
				suite.Require().NoError(err)

				escrowAddress := types.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())
				escrowBalanceAfter := suite.chainA.GetSimApp().BankKeeper.GetBalance(suite.chainA.GetContext(), escrowAddress, sdk.DefaultBondDenom)
				suite.Require().Equal(sdkmath.NewInt(0), escrowBalanceAfter.Amount)
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.expError.Error())
			}
		})
	}
}

func (suite *TransferTestSuite) TestPacketDataUnmarshalerInterface() {
	var (
		sender   = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()
		receiver = sdk.AccAddress(secp256k1.GenPrivKey().PubKey().Address()).String()

		data          []byte
		expPacketData types.FungibleTokenPacketData
	)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"success: valid packet data with memo",
			func() {
				expPacketData = types.FungibleTokenPacketData{
					Denom:    ibctesting.TestCoin.Denom,
					Amount:   ibctesting.TestCoin.Amount.String(),
					Sender:   sender,
					Receiver: receiver,
					Memo:     "some memo",
				}
				data = expPacketData.GetBytes()
			},
			true,
		},
		{
			"success: valid packet data without memo",
			func() {
				expPacketData = types.FungibleTokenPacketData{
					Denom:    ibctesting.TestCoin.Denom,
					Amount:   ibctesting.TestCoin.Amount.String(),
					Sender:   sender,
					Receiver: receiver,
					Memo:     "",
				}
				data = expPacketData.GetBytes()
			},
			true,
		},
		{
			"failure: invalid packet data",
			func() {
				data = []byte("invalid packet data")
			},
			false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		suite.Run(tc.name, func() {
			tc.malleate()

			packetData, err := transfer.IBCModule{}.UnmarshalPacketData(data)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(expPacketData, packetData)
			} else {
				suite.Require().Error(err)
				suite.Require().Nil(packetData)
			}
		})
	}
}

/*
Copyright IBM Corp. 2016 All Rights Reserved.

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

package validation

import (
	"fmt"

	"bytes"

	mspmgmt "github.com/hyperledger/fabric/msp/mgmt"
	"github.com/hyperledger/fabric/protos/common"
	pb "github.com/hyperledger/fabric/protos/peer"
	"github.com/hyperledger/fabric/protos/utils"
	"github.com/op/go-logging"
)

var putilsLogger = logging.MustGetLogger("protoutils")

// validateChaincodeProposalMessage checks the validity of a Proposal message of type CHAINCODE
func validateChaincodeProposalMessage(prop *pb.Proposal, hdr *common.Header) (*pb.ChaincodeHeaderExtension, error) {
	putilsLogger.Infof("validateChaincodeProposalMessage starts for proposal %p, header %p", prop, hdr)

	// 4) based on the header type (assuming it's CHAINCODE), look at the extensions
	chaincodeHdrExt, err := utils.GetChaincodeHeaderExtension(hdr)
	if err != nil {
		return nil, fmt.Errorf("Invalid header extension for type CHAINCODE")
	}

	putilsLogger.Infof("validateChaincodeProposalMessage info: header extension references chaincode %s", chaincodeHdrExt.ChaincodeId)

	//    - ensure that the chaincodeID is correct (?)
	// TODO: should we even do this? If so, using which interface?

	//    - ensure that the visibility field has some value we understand
	// currently the fabric only supports full visibility: this means that
	// there are no restrictions on which parts of the proposal payload will
	// be visible in the final transaction; this default approach requires
	// no additional instructions in the PayloadVisibility field which is
	// therefore expected to be nil; however the fabric may be extended to
	// encode more elaborate visibility mechanisms that shall be encoded in
	// this field (and handled appropriately by the peer)
	if chaincodeHdrExt.PayloadVisibility != nil {
		return nil, fmt.Errorf("Invalid payload visibility field")
	}

	return chaincodeHdrExt, nil
}

// ValidateProposalMessage checks the validity of a SignedProposal message
// this function returns Header and ChaincodeHeaderExtension messages since they
// have been unmarshalled and validated
func ValidateProposalMessage(signedProp *pb.SignedProposal) (*pb.Proposal, *common.Header, *pb.ChaincodeHeaderExtension, error) {
	putilsLogger.Infof("ValidateProposalMessage starts for signed proposal %p", signedProp)

	// extract the Proposal message from signedProp
	prop, err := utils.GetProposal(signedProp.ProposalBytes)
	if err != nil {
		return nil, nil, nil, err
	}

	// 1) look at the ProposalHeader
	hdr, err := utils.GetHeader(prop.Header)
	if err != nil {
		return nil, nil, nil, err
	}

	// validate the header
	err = validateCommonHeader(hdr)
	if err != nil {
		return nil, nil, nil, err
	}

	// validate the signature
	err = checkSignatureFromCreator(hdr.SignatureHeader.Creator, signedProp.Signature, signedProp.ProposalBytes, hdr.ChannelHeader.ChannelId)
	if err != nil {
		return nil, nil, nil, err
	}

	// TODO: ensure that creator can transact with us (some ACLs?) which set of APIs is supposed to give us this info?

	// Verify that the transaction ID has been computed properly.
	// This check is needed to ensure that the lookup into the ledger
	// for the same TxID catches duplicates.
	err = utils.CheckProposalTxID(
		hdr.ChannelHeader.TxId,
		hdr.SignatureHeader.Nonce,
		hdr.SignatureHeader.Creator)
	if err != nil {
		return nil, nil, nil, err
	}

	// continue the validation in a way that depends on the type specified in the header
	switch common.HeaderType(hdr.ChannelHeader.Type) {
	case common.HeaderType_CONFIG:
		//which the types are different the validation is the same
		//viz, validate a proposal to a chaincode. If we need other
		//special validation for confguration, we would have to implement
		//special validation
		fallthrough
	case common.HeaderType_ENDORSER_TRANSACTION:
		// validation of the proposal message knowing it's of type CHAINCODE
		chaincodeHdrExt, err := validateChaincodeProposalMessage(prop, hdr)
		if err != nil {
			return nil, nil, nil, err
		}

		return prop, hdr, chaincodeHdrExt, err
	default:
		//NOTE : we proably need a case
		return nil, nil, nil, fmt.Errorf("Unsupported proposal type %d", common.HeaderType(hdr.ChannelHeader.Type))
	}
}

// given a creator, a message and a signature,
// this function returns nil if the creator
// is a valid cert and the signature is valid
func checkSignatureFromCreator(creatorBytes []byte, sig []byte, msg []byte, ChainID string) error {
	putilsLogger.Infof("checkSignatureFromCreator starts")

	// check for nil argument
	if creatorBytes == nil || sig == nil || msg == nil {
		return fmt.Errorf("Nil arguments")
	}

	mspObj := mspmgmt.GetIdentityDeserializer(ChainID)
	if mspObj == nil {
		return fmt.Errorf("could not get msp for chain [%s]", ChainID)
	}

	// get the identity of the creator
	creator, err := mspObj.DeserializeIdentity(creatorBytes)
	if err != nil {
		return fmt.Errorf("Failed to deserialize creator identity, err %s", err)
	}

	putilsLogger.Infof("checkSignatureFromCreator info: creator is %s", creator.GetIdentifier())

	// ensure that creator is a valid certificate
	err = creator.Validate()
	if err != nil {
		return fmt.Errorf("The creator certificate is not valid, err %s", err)
	}

	putilsLogger.Infof("checkSignatureFromCreator info: creator is valid")

	// validate the signature
	err = creator.Verify(msg, sig)
	if err != nil {
		return fmt.Errorf("The creator's signature over the proposal is not valid, err %s", err)
	}

	putilsLogger.Infof("checkSignatureFromCreator exists successfully")

	return nil
}

// checks for a valid SignatureHeader
func validateSignatureHeader(sHdr *common.SignatureHeader) error {
	// check for nil argument
	if sHdr == nil {
		return fmt.Errorf("Nil SignatureHeader provided")
	}

	// ensure that there is a nonce
	if sHdr.Nonce == nil || len(sHdr.Nonce) == 0 {
		return fmt.Errorf("Invalid nonce specified in the header")
	}

	// ensure that there is a creator
	if sHdr.Creator == nil || len(sHdr.Creator) == 0 {
		return fmt.Errorf("Invalid creator specified in the header")
	}

	return nil
}

// checks for a valid ChannelHeader
func validateChannelHeader(cHdr *common.ChannelHeader) error {
	// check for nil argument
	if cHdr == nil {
		return fmt.Errorf("Nil ChannelHeader provided")
	}

	// validate the header type
	if common.HeaderType(cHdr.Type) != common.HeaderType_ENDORSER_TRANSACTION &&
		common.HeaderType(cHdr.Type) != common.HeaderType_CONFIG_UPDATE &&
		common.HeaderType(cHdr.Type) != common.HeaderType_CONFIG {
		return fmt.Errorf("invalid header type %s", common.HeaderType(cHdr.Type))
	}

	putilsLogger.Infof("validateChannelHeader info: header type %d", common.HeaderType(cHdr.Type))

	// TODO: validate chainID in cHdr.ChainID

	// Validate epoch in cHdr.Epoch
	// Currently we enforce that Epoch is 0.
	// TODO: This check will be modified once the Epoch management
	// will be in place.
	if cHdr.Epoch != 0 {
		return fmt.Errorf("Invalid Epoch in ChannelHeader. It must be 0. It was [%d]", cHdr.Epoch)
	}

	// TODO: Validate version in cHdr.Version

	return nil
}

// checks for a valid Header
func validateCommonHeader(hdr *common.Header) error {
	if hdr == nil {
		return fmt.Errorf("Nil header")
	}

	err := validateChannelHeader(hdr.ChannelHeader)
	if err != nil {
		return err
	}

	err = validateSignatureHeader(hdr.SignatureHeader)
	if err != nil {
		return err
	}

	return nil
}

// validateConfigTransaction validates the payload of a
// transaction assuming its type is CONFIG
func validateConfigTransaction(data []byte, hdr *common.Header) error {
	putilsLogger.Infof("validateConfigTransaction starts for data %p, header %s", data, hdr)

	// check for nil argument
	if data == nil || hdr == nil {
		return fmt.Errorf("Nil arguments")
	}

	// There is no need to do this validation here, the configtx.Manager handles this

	return nil
}

// validateEndorserTransaction validates the payload of a
// transaction assuming its type is ENDORSER_TRANSACTION
func validateEndorserTransaction(data []byte, hdr *common.Header) error {
	putilsLogger.Infof("validateEndorserTransaction starts for data %p, header %s", data, hdr)

	// check for nil argument
	if data == nil || hdr == nil {
		return fmt.Errorf("Nil arguments")
	}

	// if the type is ENDORSER_TRANSACTION we unmarshal a Transaction message
	tx, err := utils.GetTransaction(data)
	if err != nil {
		return err
	}

	// check for nil argument
	if tx == nil {
		return fmt.Errorf("Nil transaction")
	}

	// TODO: validate tx.Version

	// TODO: validate ChaincodeHeaderExtension

	if len(tx.Actions) == 0 {
		return fmt.Errorf("At least one TransactionAction is required")
	}

	putilsLogger.Infof("validateEndorserTransaction info: there are %d actions", len(tx.Actions))

	for _, act := range tx.Actions {
		// check for nil argument
		if act == nil {
			return fmt.Errorf("Nil action")
		}

		// if the type is ENDORSER_TRANSACTION we unmarshal a SignatureHeader
		sHdr, err := utils.GetSignatureHeader(act.Header)
		if err != nil {
			return err
		}

		// validate the SignatureHeader - here we actually only
		// care about the nonce since the creator is in the outer header
		err = validateSignatureHeader(sHdr)
		if err != nil {
			return err
		}

		putilsLogger.Infof("validateEndorserTransaction info: signature header is valid")

		// if the type is ENDORSER_TRANSACTION we unmarshal a ChaincodeActionPayload
		cap, err := utils.GetChaincodeActionPayload(act.Payload)
		if err != nil {
			return err
		}

		// extract the proposal response payload
		prp, err := utils.GetProposalResponsePayload(cap.Action.ProposalResponsePayload)
		if err != nil {
			return err
		}

		// build the original header by stitching together
		// the common ChannelHeader and the per-action SignatureHeader
		hdrOrig := &common.Header{ChannelHeader: hdr.ChannelHeader, SignatureHeader: sHdr}
		hdrBytes, err := utils.GetBytesHeader(hdrOrig) // FIXME: here we hope that hdrBytes will be the same one that the endorser had
		if err != nil {
			return err
		}

		// compute proposalHash
		pHash, err := utils.GetProposalHash2(hdrBytes, cap.ChaincodeProposalPayload)
		if err != nil {
			return err
		}

		// ensure that the proposal hash matches
		if bytes.Compare(pHash, prp.ProposalHash) != 0 {
			return fmt.Errorf("proposal hash does not match")
		}
	}

	return nil
}

// ValidateTransaction checks that the transaction envelope is properly formed
func ValidateTransaction(e *common.Envelope) (*common.Payload, error) {
	putilsLogger.Infof("ValidateTransactionEnvelope starts for envelope %p", e)

	// check for nil argument
	if e == nil {
		return nil, fmt.Errorf("Nil Envelope")
	}

	// get the payload from the envelope
	payload, err := utils.GetPayload(e)
	if err != nil {
		return nil, fmt.Errorf("Could not extract payload from envelope, err %s", err)
	}

	putilsLogger.Infof("Header is %s", payload.Header)

	// validate the header
	err = validateCommonHeader(payload.Header)
	if err != nil {
		return nil, err
	}

	// validate the signature in the envelope
	err = checkSignatureFromCreator(payload.Header.SignatureHeader.Creator, e.Signature, e.Payload, payload.Header.ChannelHeader.ChannelId)
	if err != nil {
		return nil, err
	}

	// TODO: ensure that creator can transact with us (some ACLs?) which set of APIs is supposed to give us this info?

	// continue the validation in a way that depends on the type specified in the header
	switch common.HeaderType(payload.Header.ChannelHeader.Type) {
	case common.HeaderType_ENDORSER_TRANSACTION:
		// Verify that the transaction ID has been computed properly.
		// This check is needed to ensure that the lookup into the ledger
		// for the same TxID catches duplicates.
		err = utils.CheckProposalTxID(
			payload.Header.ChannelHeader.TxId,
			payload.Header.SignatureHeader.Nonce,
			payload.Header.SignatureHeader.Creator)
		if err != nil {
			return nil, err
		}

		err = validateEndorserTransaction(payload.Data, payload.Header)
		putilsLogger.Infof("ValidateTransactionEnvelope returns err %s", err)
		return payload, err
	case common.HeaderType_CONFIG:
		// Config transactions have signatures inside which will be validated, especially at genesis there may be no creator or
		// signature on the outermost envelope

		err = validateConfigTransaction(payload.Data, payload.Header)
		putilsLogger.Infof("ValidateTransactionEnvelope returns err %s", err)
		return payload, err
	default:
		return nil, fmt.Errorf("Unsupported transaction payload type %d", common.HeaderType(payload.Header.ChannelHeader.Type))
	}
}

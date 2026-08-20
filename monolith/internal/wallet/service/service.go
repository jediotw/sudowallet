package service

import (
	"context"
	customErr "github.com/saurabhkr78/sudowallet/monolith/internal/errors"
	walletModel "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/model"
	walletRepo "github.com/saurabhkr78/sudowallet/monolith/internal/wallet/repository"
)

/*
WalletService defines what wallet operations are available, walletService stores the repository it needs, and NewWalletService() injects that repository into the service.
*/
// all the wallet operations?
type WalletService interface {
	GetwalletByUserID(ctx context.Context, userID string) (*walletModel.Wallet, error)
}

// store the wallet repository in the service struct so that we can use it in the service methods
type walletService struct {
	walletRepo walletRepo.WalletRepository
}

// constructore to create a new wallet service and inject the wallet repository into it
func NewWalletService(wRepo walletRepo.WalletRepository) WalletService {
	return &walletService{walletRepo: wRepo}
}

// implement the GetwalletByUserID method of the WalletService interface, this method will call the GetByUserID method of the wallet repository to get the wallet by user id
// this is the method attached to the walletService struct that implements the WalletService interface, it takes a context and a user id as parameters and returns a wallet model and an error
func (s *walletService) GetwalletByUserID(ctx context.Context, userID string) (*walletModel.Wallet, error) {
	//get the user by id from the repository, this will return a wallet model and an error
	w, err := s.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, customErr.NewAppError(500, "WALLET_NOT_FOUND", "Wallet not found ")
	}
	return w, nil
}

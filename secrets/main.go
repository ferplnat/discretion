package secrets

import (
	"context"
	"errors"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/keyvault/azsecrets"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/keyvault/armkeyvault"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/subscription/armsubscription"
	"github.com/google/uuid"
)

var secrets = map[string]*SecretInfo{}
var vaults = []*Vault{}
var credential azcore.TokenCredential

func Init() error {
	var err error
	credential, err = azidentity.NewAzureCLICredential(nil)
	if err != nil {
		return err
	}

	subscriptions, err := getSubscriptions()
	if err != nil {
		return err
	}

	go getVaults(subscriptions)

	return nil
}

func GetVaults() []*Vault {
	return vaults
}

func GetSecrets() map[string]*SecretInfo {
	return secrets
}

func TryGetSecret(version string) (*SecretInfo, bool) {
	secretInfo := secrets[version]
	if secretInfo == nil {
		return nil, false
	}

	secretsClient, err := azsecrets.NewClient(secretInfo.VaultUrl, credential, nil)
	if err != nil {
		return nil, false
	}

	secretValue, err := secretsClient.GetSecret(context.Background(), secretInfo.Name, secretInfo.Version, nil)

	if err != nil {
		secretInfo.Value = ""
	} else {
		secretInfo.Value = *secretValue.Value
	}

	return secretInfo, true
}

func registerVaults(subscription string, vaults []*armkeyvault.Resource) {
	for _, vault := range vaults {
		registerVault(subscription, vault)
	}
}

func registerVault(subscription string, v *armkeyvault.Resource) {
	vault := &Vault{
		Subscription:  subscription,
		ResourceGroup: getResourceGroupFromId(*v.ID),
		ID:            *v.ID,
		Name:          *v.Name,
		Region:        *v.Location,
		Url:           "https://" + *v.Name + ".vault.azure.net",
	}

	vaults = append(vaults, vault)
	err := getSecretsInfo(vault)
	if err != nil {
		panic(err)
	}
}

func getResourceGroupFromId(id string) string {
	parts := strings.Split(id, "/")
	return parts[4]
}

func getSubscriptions() ([]*armsubscription.Subscription, error) {
	var subscriptions []*armsubscription.Subscription
	subscriptionClient, err := armsubscription.NewSubscriptionsClient(credential, nil)

	if err != nil {
		return subscriptions, err
	}

	listPager := subscriptionClient.NewListPager(nil)
	for listPager.More() {
		page, err := listPager.NextPage(context.Background())
		if err != nil {
			return subscriptions, err
		}

		subscriptions = append(subscriptions, page.Value...)
	}

	return subscriptions, nil
}

func getVaults(subscriptions []*armsubscription.Subscription) error {
	for _, subscription := range subscriptions {
		vaultsClient, err := armkeyvault.NewVaultsClient(*subscription.SubscriptionID, credential, nil)
		if err != nil {
			return err
		}

		listPager := vaultsClient.NewListPager(nil)
		for listPager.More() {
			page, err := listPager.NextPage(context.Background())
			if err != nil {
				return err
			}

			registerVaults(*subscription.SubscriptionID, page.Value)
		}
	}

	return nil
}

func getSecretsInfo(vault *Vault) error {
	secretsClient, err := azsecrets.NewClient(vault.Url, credential, nil)
	if err != nil {
		// TODO: Log to file
		return nil
	}

	listPager := secretsClient.NewListSecretsPager(nil)

	for listPager.More() {
		page, err := listPager.NextPage(context.Background())
		if err != nil {
			var e *azcore.ResponseError
			if errors.As(err, &e); e != nil && e.StatusCode == 403 {
				break
			} else {
				// TODO: Log to file
				return nil
			}
		}

		for _, secret := range page.Value {
			secretInfo := &SecretInfo{
				Vault:      vault.Name,
				VaultUrl:   vault.Url,
				Name:       secret.ID.Name(),
				Enabled:    *secret.Attributes.Enabled,
				Version:    secret.ID.Version(),
				Identifier: uuid.New().String(),
			}

			secrets[secretInfo.Identifier] = secretInfo
		}
	}

	return nil
}

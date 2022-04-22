package util

import "github.com/spf13/viper"

type Config struct {
	EthUri           string `mapstructure:"ETH_URI"`
	MongoUri         string `mapstructure:"MONGO_URI"`
	MongoDb          string `mapstructure:"MONGO_DB"`
	MongoEvent       string `mapstructure:"MONGO_EVENT_COLLECTION"`
	MongoNft         string `mapstructure:"MONGO_NFT_COLLECTION"`
	MongoApprovedNft string `mapstructure:"MONGO_APPROVED_COLLECTION"`
	MongoBlock       string `mapstructure:"MONGO_BLOCK_COLLECTION"`
}

func LoadConfig() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.AutomaticEnv()

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	config := &Config{}
	err = viper.Unmarshal(config)
	if err != nil {
		return nil, err
	}
	return config, nil
}

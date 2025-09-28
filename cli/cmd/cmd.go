package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/unitechio/gohtml/client"
	"github.com/unitechio/gohtml/content"
	"github.com/unitechio/gohtml/sizes"
	"github.com/unitechio/gopdf/common"
)

var cfgFile string
var (
	generateCfg = generateConfig{}
	paramsCfg   = parametersConfig{
		PaperWidth:   sizes.LengthFlag{Length: sizes.Inch(8.5).Millimeters()},
		PaperHeight:  sizes.LengthFlag{Length: sizes.Inch(11).Millimeters()},
		Orientation:  sizes.Portrait,
		MarginTop:    sizes.LengthFlag{Length: sizes.Millimeter(10)},
		MarginBottom: sizes.LengthFlag{Length: sizes.Millimeter(10)},
		MarginLeft:   sizes.LengthFlag{Length: sizes.Millimeter(10)},
		MarginRight:  sizes.LengthFlag{Length: sizes.Millimeter(10)},
	}
)
var (
	debug   bool
	verbose bool
)

type parametersConfig struct {
	PaperWidth   sizes.LengthFlag  `mapstructure:"paper-width"`
	PaperHeight  sizes.LengthFlag  `mapstructure:"paper-height"`
	PageSize     sizes.PageSize    `mapstructure:"page-size"`
	Orientation  sizes.Orientation `mapstructure:"orientation"`
	MarginTop    sizes.LengthFlag  `mapstructure:"margin-top"`
	MarginBottom sizes.LengthFlag  `mapstructure:"margin-bottom"`
	MarginLeft   sizes.LengthFlag  `mapstructure:"margin-left"`
	MarginRight  sizes.LengthFlag  `mapstructure:"margin-right"`
}

type generateConfig struct {
	Port   int    `mapstructure:"port"`
	Host   string `mapstructure:"host"`
	Https  bool   `mapstructure:"https"`
	Prefix string `mapstructure:"prefix"`
}

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generates PDF based on the provided HTML or directory with the HTML files.",
	Long: `A longer description that spans multiple lines and likely contains examples and usage of using your command. 
			For example: Cobra is a CLI library for Go that empowers applications.
			This application is a tool to generate the needed files
			to quickly create a Cobra application.`,
	Run:        runGenerate,
	Args:       cobra.ExactArgs(2),
	ArgAliases: []string{"input", "output-pdf"},
	Example:    "generate input.html output.pdf --orientation portrait",
}

var rootCmd = &cobra.Command{
	Use:   "unihtml",
	Short: "A brief description of your application",
	Long: `A longer description that spans multiple lines and likely contains
			examples and usage of using your application. For example:

			Cobra is a CLI library for Go that empowers applications.
			This application is a tool to generate the needed files
			to quickly create a Cobra application.`,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().IntP("port", "p", 8080, "Port of the unihtml server")
	generateCmd.Flags().String("host", "localhost", "Host name of the unihtml server")
	generateCmd.Flags().BoolP("https", "s", false, "Protocol used in server communication")
	generateCmd.Flags().StringP("prefix", "x", "", "Public api prefix used by the unihtml server")
	generateCmd.Flags().Var(&paramsCfg.PaperWidth, "paper-width", "sets up the paper-width")
	generateCmd.Flags().Var(&paramsCfg.PaperHeight, "paper-height", "sets up the paper-height")
	generateCmd.Flags().Var(&paramsCfg.PageSize, "paper-size", "sets up the page size")
	generateCmd.Flags().Var(&paramsCfg.Orientation, "orientation", "sets up the page orientation")
	generateCmd.Flags().Var(&paramsCfg.MarginTop, "margin-top", "sets up the margin-top")
	generateCmd.Flags().Var(&paramsCfg.MarginBottom, "margin-bottom", "sets up the margin-bottom")
	generateCmd.Flags().Var(&paramsCfg.MarginRight, "margin-right", "sets up the margin-right")
	generateCmd.Flags().Var(&paramsCfg.MarginLeft, "margin-left", "sets up the margin-left")

	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Defines debug mode")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose information of the client")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.unihtml-src.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		viper.AddConfigPath(home)
		viper.SetConfigName(".unihtml-src")
	}
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}

func runGenerate(cmd *cobra.Command, args []string) {
	start := time.Now()

	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}
	if err := viper.Unmarshal(&generateCfg); err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	setupLogging()

	inputStat, err := os.Stat(args[0])
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}
	if !inputStat.IsDir() && filepath.Ext(inputStat.Name()) != ".html" {
		fmt.Printf("Err: Currently only HTML files inputStat are supported. Input: %s", args[0])
		os.Exit(1)
	}

	outputFile, err := os.OpenFile(args[1], os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0700)
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}
	defer outputFile.Close()

	clientObj := client.New(client.Options{
		HTTPS:    generateCfg.Https,
		Hostname: generateCfg.Host,
		Port:     generateCfg.Port,
		Prefix:   generateCfg.Prefix,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var contentObj content.Content
	if inputStat.IsDir() {
		contentObj, err = content.NewZipDirectory(args[0])
	} else {
		contentObj, err = content.NewHTMLFile(args[0])
	}
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	query, err := client.BuildHTMLQuery().
		PaperWidth(paramsCfg.PaperWidth.Length).
		PaperHeight(paramsCfg.PaperHeight.Length).
		PageSize(paramsCfg.PageSize).
		MarginTop(paramsCfg.MarginTop.Length).
		MarginBottom(paramsCfg.MarginBottom.Length).
		MarginLeft(paramsCfg.MarginLeft.Length).
		MarginRight(paramsCfg.MarginRight.Length).
		Orientation(paramsCfg.Orientation).
		SetContent(contentObj).
		Query()
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	resp, err := clientObj.ConvertHTML(ctx, query)
	if err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	common.Log.Trace("Executing generate query taken: %s", time.Since(start))
	start = time.Now()

	if _, err = outputFile.Write(resp.Data); err != nil {
		fmt.Printf("Err: %v", err)
		os.Exit(1)
	}

	common.Log.Trace("Writing file taken: %s", time.Since(start))
	fmt.Printf("Generated with success in %s", time.Since(start))
}

func setupLogging() {
	level := common.LogLevelInfo
	if debug {
		level = common.LogLevelDebug
	}
	if verbose {
		level = common.LogLevelTrace
	}
	common.Log = common.NewConsoleLogger(level)
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

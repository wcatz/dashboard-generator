package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/wcatz/dashboard-generator/internal/config"
	"github.com/wcatz/dashboard-generator/internal/generator"
	"github.com/wcatz/dashboard-generator/internal/server"
	"github.com/wcatz/dashboard-generator/web"
)

var (
	// build-time version info (injected via ldflags)
	version   = "dev"
	commit    = "none"
	buildDate = "unknown"

	// CLI flags
	cfgFile         string
	profile         string
	outputDir       string
	prometheusURL   string
	prometheusUser  string
	prometheusPass  string
	prometheusToken string
	grafanaURL      string
	grafanaUser     string
	grafanaPass     string
	grafanaToken    string
	dryRun          bool
	verbose         bool
	servePort       int
	outputFormat    string
	k8sNamespace    string
	grafanaFolder   string
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "dashboard-generator",
		Short:   "config-driven Grafana dashboard generator",
		Version: formatVersion(),
	}

	genCmd := &cobra.Command{
		Use:   "generate",
		Short: "generate Grafana dashboard JSON from YAML config",
		RunE:  runGenerate,
	}
	genCmd.Flags().StringVar(&cfgFile, "config", "", "path to YAML config file (required)")
	genCmd.Flags().StringVar(&profile, "profile", "", "generate only dashboards in named profile")
	genCmd.Flags().StringVar(&outputDir, "output-dir", "", "override output directory")
	genCmd.Flags().BoolVar(&dryRun, "dry-run", false, "generate to memory only")
	genCmd.Flags().BoolVar(&verbose, "verbose", false, "print panel details")
	genCmd.Flags().StringVar(&outputFormat, "format", "json", "output format: json, grafana-yaml, configmap (comma-separated for multiple)")
	genCmd.Flags().StringVar(&k8sNamespace, "k8s-namespace", "monitoring", "Kubernetes namespace for YAML outputs")
	genCmd.Flags().StringVar(&grafanaFolder, "grafana-folder", "", "Grafana folder for dashboards")
	_ = genCmd.MarkFlagRequired("config")

	discoverCmd := &cobra.Command{
		Use:   "discover",
		Short: "query Prometheus and print suggested YAML config",
		RunE:  runDiscover,
	}
	discoverCmd.Flags().StringVar(&cfgFile, "config", "", "path to YAML config file (required)")
	discoverCmd.Flags().StringVar(&prometheusURL, "prometheus-url", "", "Prometheus URL for discovery")
	discoverCmd.Flags().StringVar(&prometheusUser, "prometheus-user", "", "Prometheus basic auth username")
	discoverCmd.Flags().StringVar(&prometheusPass, "prometheus-pass", "", "Prometheus basic auth password")
	discoverCmd.Flags().StringVar(&prometheusToken, "prometheus-token", "", "Prometheus bearer token")
	_ = discoverCmd.MarkFlagRequired("config")

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "generate and push dashboards to Grafana API",
		RunE:  runPush,
	}
	pushCmd.Flags().StringVar(&cfgFile, "config", "", "path to YAML config file (required)")
	pushCmd.Flags().StringVar(&profile, "profile", "", "generate only dashboards in named profile")
	pushCmd.Flags().StringVar(&outputDir, "output-dir", "", "override output directory")
	pushCmd.Flags().StringVar(&grafanaURL, "grafana-url", "", "Grafana URL (required)")
	pushCmd.Flags().StringVar(&grafanaUser, "grafana-user", "", "Grafana basic auth user")
	pushCmd.Flags().StringVar(&grafanaPass, "grafana-pass", "", "Grafana basic auth password")
	pushCmd.Flags().StringVar(&grafanaToken, "grafana-token", "", "Grafana API token")
	pushCmd.Flags().BoolVar(&verbose, "verbose", false, "print panel details")
	_ = pushCmd.MarkFlagRequired("config")
	_ = pushCmd.MarkFlagRequired("grafana-url")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "start the web UI server",
		RunE:  runServe,
	}
	serveCmd.Flags().StringVar(&cfgFile, "config", "", "path to YAML config file (required)")
	serveCmd.Flags().IntVar(&servePort, "port", 8080, "HTTP server port")
	serveCmd.Flags().StringVar(&grafanaURL, "grafana-url", "", "Grafana URL for push (or set GRAFANA_URL env)")
	serveCmd.Flags().StringVar(&grafanaToken, "grafana-token", "", "Grafana API token (or set GRAFANA_TOKEN env)")
	_ = serveCmd.MarkFlagRequired("config")

	rootCmd.AddCommand(genCmd, discoverCmd, pushCmd, serveCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func parseOutputFormats(formatStr string) []generator.OutputFormat {
	if formatStr == "" {
		formatStr = "json"
	}
	parts := strings.Split(formatStr, ",")
	formats := make([]generator.OutputFormat, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "json":
			formats = append(formats, generator.FormatJSON)
		case "grafana-yaml":
			formats = append(formats, generator.FormatGrafanaYAML)
		case "configmap":
			formats = append(formats, generator.FormatConfigMap)
		}
	}
	if len(formats) == 0 {
		formats = []generator.OutputFormat{generator.FormatJSON}
	}
	return formats
}

func loadConfig() (*config.Config, error) {
	cliArgs := make(map[string]string)
	if prometheusURL != "" {
		cliArgs["prometheus_url"] = prometheusURL
	}
	return config.Load(cfgFile, cliArgs)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return generateDashboards(cfg, false)
}

func runDiscover(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	discoveryCfg := cfg.GetDiscovery()
	sources := discoveryCfg.Sources
	if len(sources) == 0 {
		for name := range cfg.Datasources {
			sources = append(sources, name)
		}
	}
	if len(sources) == 0 {
		return fmt.Errorf("no datasources configured for discovery")
	}

	var disc *generator.MetricDiscovery
	if prometheusUser != "" || prometheusToken != "" {
		disc = generator.NewMetricDiscoveryWithAuth(cfg, prometheusUser, prometheusPass, prometheusToken)
	} else {
		disc = generator.NewMetricDiscovery(cfg)
	}
	return disc.PrintDiscovery(sources, discoveryCfg.IncludePatterns, discoveryCfg.ExcludePatterns)
}

func runServe(cmd *cobra.Command, args []string) error {
	gURL := grafanaURL
	if gURL == "" {
		gURL = os.Getenv("GRAFANA_URL")
	}
	gToken := grafanaToken
	if gToken == "" {
		gToken = os.Getenv("GRAFANA_TOKEN")
	}
	srv, err := server.New(web.EmbeddedFS, cfgFile, gURL, gToken)
	if err != nil {
		return err
	}
	addr := fmt.Sprintf(":%d", servePort)
	return srv.ListenAndServe(addr)
}

func runPush(cmd *cobra.Command, args []string) error {
	cfg, err := loadConfig()
	if err != nil {
		return err
	}
	return generateDashboards(cfg, true)
}

func generateDashboards(cfg *config.Config, push bool) error {
	gen := cfg.GetGenerator()

	// determine output directory
	outDir := outputDir
	if outDir == "" {
		outDir = gen.OutputDir
	}
	if outDir == "" {
		outDir = "."
	}
	if !filepath.IsAbs(outDir) {
		configDir := filepath.Dir(cfgFile)
		absConfig, err := filepath.Abs(configDir)
		if err != nil {
			return err
		}
		outDir = filepath.Join(absConfig, outDir)
	}
	if !dryRun {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			return err
		}
	}

	// get dashboards
	dashboards, err := cfg.GetDashboards(profile)
	if err != nil {
		return err
	}
	if len(dashboards) == 0 {
		return fmt.Errorf("no dashboards defined in config")
	}

	// get dashboard order
	order, err := cfg.GetDashboardOrder(profile)
	if err != nil {
		return err
	}
	// ensure order only includes dashboards that exist
	var filteredOrder []string
	for _, name := range order {
		if _, ok := dashboards[name]; ok {
			filteredOrder = append(filteredOrder, name)
		}
	}
	// add any dashboards not in the order list
	orderSet := make(map[string]bool)
	for _, name := range filteredOrder {
		orderSet[name] = true
	}
	var remaining []string
	for name := range dashboards {
		if !orderSet[name] {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	filteredOrder = append(filteredOrder, remaining...)

	// build components
	idGen := generator.NewIDGenerator()
	panelFactory := generator.NewPanelFactory(cfg, idGen)
	layoutEngine := generator.NewLayoutEngine()
	builder := generator.NewDashboardBuilder(cfg, panelFactory, layoutEngine)

	// build navigation links
	navLinks := builder.BuildNavigationLinks(dashboards, filteredOrder)

	// auto-discovery sections if enabled
	var discoverySections []config.SectionConfig
	discoveryCfg := cfg.GetDiscovery()
	if discoveryCfg.Enabled && len(discoveryCfg.Sources) > 0 {
		disc := generator.NewMetricDiscovery(cfg)
		discoverySections, err = disc.GenerateDiscoverySections(
			discoveryCfg.Sources,
			discoveryCfg.IncludePatterns,
			discoveryCfg.ExcludePatterns,
		)
		if err != nil {
			return fmt.Errorf("discovery: %w", err)
		}
	}

	// parse output formats
	formats := parseOutputFormats(outputFormat)

	// determine default namespace and folder
	defaultNamespace := k8sNamespace
	defaultFolder := grafanaFolder
	if defaultFolder == "" {
		// use first tag as folder, if any
		if len(dashboards) > 0 {
			for _, dbCfg := range dashboards {
				if len(dbCfg.Tags) > 0 {
					defaultFolder = dbCfg.Tags[0]
					break
				}
			}
		}
	}

	// generate dashboards
	totalSize := 0
	totalPanels := 0
	fmt.Println("grafana dashboard generator:")

	for _, name := range filteredOrder {
		dbCfg := dashboards[name]
		dashboard, err := builder.Build(dbCfg, navLinks, discoverySections)
		if err != nil {
			return fmt.Errorf("building dashboard '%s': %w", name, err)
		}

		// determine output path base (without extension)
		basePath := filepath.Join(outDir, name)
		if dbCfg.Filename != "" {
			filename := filepath.Base(dbCfg.Filename)
			if filename != "" && filename != "." {
				// remove extension if present
				ext := filepath.Ext(filename)
				if ext != "" {
					filename = filename[:len(filename)-len(ext)]
				}
				basePath = filepath.Join(outDir, filename)
			}
		}

		// determine namespace and folder for this dashboard
		namespace := defaultNamespace
		folder := defaultFolder
		if dbCfg.K8sNamespace != "" {
			namespace = dbCfg.K8sNamespace
		}
		if dbCfg.GrafanaFolder != "" {
			folder = dbCfg.GrafanaFolder
		} else if folder == "" && len(dbCfg.Tags) > 0 {
			// use first tag as folder if not set
			folder = dbCfg.Tags[0]
		}

		// write dashboard in all requested formats
		var size int
		if len(formats) == 1 && formats[0] == generator.FormatJSON {
			// backward compat: single JSON output
			size, err = generator.WriteDashboard(dashboard, basePath+".json", dryRun)
		} else {
			// multi-format output
			err = generator.WriteDashboardMultiFormat(dashboard, basePath, formats, namespace, folder, dryRun)
			size = 0 // size reporting not accurate for multi-format
		}
		if err != nil {
			return err
		}
		totalSize += size

		panels, _ := dashboard["panels"].([]interface{})
		totalPanels += len(panels)

		if verbose {
			for _, p := range panels {
				if panel, ok := p.(map[string]interface{}); ok {
					ptype := panel["type"]
					ptitle := panel["title"]
					fmt.Printf("    [%v] %v\n", ptype, ptitle)
				}
			}
		}

		if push && grafanaURL != "" {
			if err := generator.PushToGrafana(dashboard, grafanaURL, grafanaUser, grafanaPass, grafanaToken); err != nil {
				fmt.Fprintf(os.Stderr, "  error pushing %s: %v\n", name, err)
			}
		}
	}

	fmt.Printf("\n  total: %d dashboards, %d panels, %s bytes\n", len(dashboards), totalPanels, formatTotalSize(totalSize))
	return nil
}

func formatTotalSize(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var result []byte
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatVersion() string {
	if commit != "none" {
		return fmt.Sprintf("%s (commit: %s, built: %s)", version, commit[:7], buildDate)
	}
	return version
}

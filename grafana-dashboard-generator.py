#!/usr/bin/env python3
"""
General-purpose Grafana dashboard generator.

Reads a YAML config file and outputs interlinked Grafana-compatible JSON
dashboards. Works with any Prometheus-monitored infrastructure.

Dependencies: PyYAML (pip install pyyaml)

Usage:
  ./grafana-dashboard-generator.py --config config.yaml
  ./grafana-dashboard-generator.py --config config.yaml --profile my-profile
  ./grafana-dashboard-generator.py --config config.yaml --discover-print --prometheus-url http://localhost:9090
  ./grafana-dashboard-generator.py --config config.yaml --dry-run
  ./grafana-dashboard-generator.py --config config.yaml --push --grafana-url http://localhost:3000
"""
import argparse
import base64
import json
import os
import re
import sys
import urllib.request
import urllib.error
import fnmatch
from collections import defaultdict

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)


# ──────────────────────────────────────────────────────────────────────────────
# ID GENERATOR
# ──────────────────────────────────────────────────────────────────────────────

class IdGenerator:
    def __init__(self):
        self._id = 0

    def reset(self):
        self._id = 0

    def next(self):
        self._id += 1
        return self._id


# ──────────────────────────────────────────────────────────────────────────────
# CONFIG
# ──────────────────────────────────────────────────────────────────────────────

class Config:
    def __init__(self, data, cli_args=None):
        self.data = data
        self.cli_args = cli_args or {}
        self._palette = self._resolve_palette()

    @classmethod
    def load(cls, path, cli_args=None):
        with open(path) as f:
            data = yaml.safe_load(f)
        return cls(data, cli_args)

    def _resolve_palette(self):
        palettes = self.data.get("palettes", {})
        active = self.data.get("active_palette", "")
        return palettes.get(active, {})

    def get_generator(self):
        return self.data.get("generator", {})

    def get_datasource(self, name):
        ds = self.data.get("datasources", {}).get(name)
        if not ds:
            raise ValueError(f"datasource '{name}' not defined in config")
        return {"type": ds["type"], "uid": ds["uid"]}

    def get_datasource_url(self, name):
        ds = self.data.get("datasources", {}).get(name, {})
        url = ds.get("url", "")
        if self.cli_args.get("prometheus_url") and name == self._first_ds_name():
            url = self.cli_args["prometheus_url"]
        return url

    def _first_ds_name(self):
        ds = self.data.get("datasources", {})
        return next(iter(ds), "")

    def get_default_datasource(self):
        ds_map = self.data.get("datasources", {})
        for name, ds in ds_map.items():
            if ds.get("is_default"):
                return {"type": ds["type"], "uid": ds["uid"]}
        if ds_map:
            first = next(iter(ds_map.values()))
            return {"type": first["type"], "uid": first["uid"]}
        return {"type": "prometheus", "uid": "prometheus"}

    def get_thresholds(self, name):
        t = self.data.get("thresholds", {}).get(name)
        if t is None:
            return None
        resolved = []
        for step in t:
            s = dict(step)
            if isinstance(s.get("color"), str) and s["color"].startswith("$"):
                s["color"] = self._resolve_color(s["color"][1:])
            resolved.append(s)
        return resolved

    def get_selector(self, name):
        return self.data.get("selectors", {}).get(name, "")

    def get_constant(self, name):
        return str(self.data.get("constants", {}).get(name, ""))

    def get_variable_def(self, name):
        return self.data.get("variables", {}).get(name)

    def get_dashboards(self, profile=None):
        all_dbs = self.data.get("dashboards", {})
        if profile:
            p = self.data.get("profiles", {}).get(profile)
            if not p:
                raise ValueError(f"profile '{profile}' not defined in config")
            names = p.get("dashboards", [])
            return {k: v for k, v in all_dbs.items() if k in names}
        return all_dbs

    def get_discovery(self):
        return self.data.get("discovery", {})

    def _resolve_color(self, name):
        return self._palette.get(name, name)

    def resolve_ref(self, value):
        """Resolve $references and ${references} in a string value."""
        if not isinstance(value, str):
            return value

        # resolve ${name} references (constants and selectors)
        def replace_braced(m):
            ref_name = m.group(1)
            c = self.get_constant(ref_name)
            if c:
                return c
            s = self.get_selector(ref_name)
            if s:
                return s
            return m.group(0)

        value = re.sub(r'\$\{(\w+)\}', replace_braced, value)
        return value

    def resolve_color(self, value):
        """Resolve a $color_name reference to a hex color."""
        if isinstance(value, str) and value.startswith("$"):
            return self._resolve_color(value[1:])
        return value

    def resolve_thresholds(self, value):
        """Resolve a $threshold_name or inline threshold list."""
        if isinstance(value, str) and value.startswith("$"):
            return self.get_thresholds(value[1:])
        if isinstance(value, list):
            resolved = []
            for step in value:
                s = dict(step)
                if isinstance(s.get("color"), str) and s["color"].startswith("$"):
                    s["color"] = self._resolve_color(s["color"][1:])
                resolved.append(s)
            return resolved
        return value


# ──────────────────────────────────────────────────────────────────────────────
# LAYOUT ENGINE
# ──────────────────────────────────────────────────────────────────────────────

class LayoutEngine:
    def __init__(self, grid_width=24):
        self.grid_width = grid_width
        self.cursor_x = 0
        self.cursor_y = 0
        self.row_height = 0

    def reset(self):
        self.cursor_x = 0
        self.cursor_y = 0
        self.row_height = 0

    def add_row(self):
        if self.cursor_x > 0:
            self.cursor_y += self.row_height
            self.cursor_x = 0
            self.row_height = 0
        y = self.cursor_y
        self.cursor_y += 1
        self.cursor_x = 0
        self.row_height = 0
        return y

    def place(self, width, height):
        if self.cursor_x + width > self.grid_width:
            self.cursor_y += self.row_height
            self.cursor_x = 0
            self.row_height = 0
        x = self.cursor_x
        y = self.cursor_y
        self.cursor_x += width
        self.row_height = max(self.row_height, height)
        return x, y

    def finish_section(self):
        if self.cursor_x > 0:
            self.cursor_y += self.row_height
            self.cursor_x = 0
            self.row_height = 0


# ──────────────────────────────────────────────────────────────────────────────
# PANEL FACTORY
# ──────────────────────────────────────────────────────────────────────────────

# default sizes per panel type
DEFAULT_SIZES = {
    "stat": (3, 4),
    "gauge": (3, 4),
    "timeseries": (12, 7),
    "bargauge": (6, 5),
    "heatmap": (12, 8),
    "histogram": (12, 7),
    "table": (24, 8),
    "piechart": (6, 6),
    "state-timeline": (12, 5),
    "status-history": (12, 5),
    "text": (24, 3),
    "logs": (24, 8),
    "row": (24, 1),
    "comparison": (12, 8),
}


class PanelFactory:
    def __init__(self, config, id_gen):
        self.config = config
        self.id_gen = id_gen

    def _ds(self, panel_cfg):
        ds_name = panel_cfg.get("datasource")
        if ds_name:
            return self.config.get_datasource(ds_name)
        return self.config.get_default_datasource()

    def _target(self, expr, legend="{{instance}}", ref_id="A", datasource=None):
        ds = datasource or self.config.get_default_datasource()
        return {
            "datasource": ds,
            "editorMode": "code",
            "expr": self.config.resolve_ref(expr),
            "legendFormat": legend,
            "range": True,
            "refId": ref_id,
        }

    def _build_targets(self, panel_cfg, datasource=None):
        targets = []
        ds = datasource or self._ds(panel_cfg)

        if "query" in panel_cfg:
            legend = panel_cfg.get("legend", "{{instance}}")
            targets.append(self._target(panel_cfg["query"], legend, "A", ds))

        if "targets" in panel_cfg:
            for i, t in enumerate(panel_cfg["targets"]):
                t_ds = ds
                if "datasource" in t:
                    t_ds = self.config.get_datasource(t["datasource"])
                legend = t.get("legend", "{{instance}}")
                ref = chr(65 + i)
                targets.append(self._target(t["expr"], legend, ref, t_ds))

        return targets

    def _thresholds(self, panel_cfg, default_color=None):
        t = panel_cfg.get("thresholds")
        if t:
            resolved = self.config.resolve_thresholds(t)
            if resolved:
                return resolved
        color = default_color or "#73BF69"
        if panel_cfg.get("color"):
            color = self.config.resolve_color(panel_cfg["color"])
        return [{"color": color, "value": None}]

    def _overrides(self, panel_cfg):
        return panel_cfg.get("overrides", [])

    def _value_mappings(self, panel_cfg):
        return panel_cfg.get("value_mappings", [])

    def _data_links(self, panel_cfg):
        return panel_cfg.get("data_links", [])

    def from_config(self, panel_cfg, x, y):
        ptype = panel_cfg["type"]
        dispatch = {
            "stat": self.stat,
            "gauge": self.gauge,
            "timeseries": self.timeseries,
            "bargauge": self.bargauge,
            "heatmap": self.heatmap,
            "histogram": self.histogram,
            "table": self.table,
            "piechart": self.piechart,
            "state-timeline": self.state_timeline,
            "status-history": self.status_history,
            "text": self.text,
            "logs": self.logs,
        }
        fn = dispatch.get(ptype)
        if not fn:
            raise ValueError(f"unknown panel type: {ptype}")
        return fn(panel_cfg, x, y)

    def row(self, title, y, collapsed=False, panels=None, repeat=None):
        r = {
            "collapsed": collapsed,
            "gridPos": {"h": 1, "w": 24, "x": 0, "y": y},
            "id": self.id_gen.next(),
            "panels": panels or [],
            "title": title,
            "type": "row",
        }
        if repeat:
            r["repeat"] = repeat
            r["repeatDirection"] = "h"
        return r

    def stat(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["stat"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        steps = self._thresholds(cfg)
        color = self.config.resolve_color(cfg.get("color", ""))
        if color and len(steps) == 1:
            steps = [{"color": color, "value": None}]
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "thresholds"},
                    "mappings": self._value_mappings(cfg),
                    "thresholds": {"mode": "absolute", "steps": steps},
                    "unit": cfg.get("unit", "none"),
                    "links": self._data_links(cfg),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "colorMode": cfg.get("color_mode", "background"),
                "graphMode": cfg.get("graph_mode", "none"),
                "justifyMode": "center",
                "orientation": "auto",
                "reduceOptions": {
                    "calcs": cfg.get("calcs", ["lastNotNull"]),
                    "fields": "", "values": False,
                },
                "showPercentChange": False,
                "textMode": cfg.get("text_mode", "value_and_name"),
                "wideLayout": True,
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "stat",
        }

    def gauge(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["gauge"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "thresholds"},
                    "mappings": self._value_mappings(cfg),
                    "max": cfg.get("max", 100),
                    "min": cfg.get("min", 0),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "percent"),
                    "links": self._data_links(cfg),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "minVizHeight": 75,
                "minVizWidth": 75,
                "orientation": "auto",
                "reduceOptions": {
                    "calcs": cfg.get("calcs", ["lastNotNull"]),
                    "fields": "", "values": False,
                },
                "showThresholdLabels": cfg.get("show_threshold_labels", False),
                "showThresholdMarkers": cfg.get("show_threshold_markers", True),
                "sizing": "auto",
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "gauge",
        }

    def timeseries(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["timeseries"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        fill = cfg.get("fill_opacity", 8)
        line = cfg.get("line_width", 1)
        stack = cfg.get("stack", "none")
        draw = cfg.get("draw_style", "line")
        interpolation = cfg.get("line_interpolation", "smooth")
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": cfg.get("color_mode", "palette-classic-by-name")},
                    "custom": {
                        "axisBorderShow": False,
                        "axisCenteredZero": False,
                        "axisColorMode": "text",
                        "axisLabel": cfg.get("axis_label", ""),
                        "axisPlacement": "auto",
                        "barAlignment": 0,
                        "barWidthFactor": 0.6,
                        "drawStyle": draw,
                        "fillOpacity": fill,
                        "gradientMode": "scheme",
                        "hideFrom": {"legend": False, "tooltip": False, "viz": False},
                        "insertNulls": False,
                        "lineInterpolation": interpolation,
                        "lineWidth": line,
                        "pointSize": 5,
                        "scaleDistribution": {"type": "linear"},
                        "showPoints": "never",
                        "spanNulls": False,
                        "stacking": {"group": "A", "mode": stack},
                        "thresholdsStyle": {"mode": "off"},
                    },
                    "mappings": self._value_mappings(cfg),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "short"),
                    "links": self._data_links(cfg),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "legend": {
                    "calcs": cfg.get("legend_calcs", []),
                    "displayMode": cfg.get("legend_mode", "list"),
                    "placement": cfg.get("legend_placement", "bottom"),
                    "showLegend": cfg.get("show_legend", True),
                },
                "tooltip": {"mode": "multi", "sort": "desc"},
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "timeseries",
        }

    def bargauge(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["bargauge"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "thresholds"},
                    "mappings": self._value_mappings(cfg),
                    "max": cfg.get("max", 100),
                    "min": cfg.get("min", 0),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "percent"),
                    "links": self._data_links(cfg),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "displayMode": cfg.get("display_mode", "gradient"),
                "maxVizHeight": 300,
                "minVizHeight": 16,
                "minVizWidth": 8,
                "namePlacement": "auto",
                "orientation": cfg.get("orientation", "horizontal"),
                "reduceOptions": {
                    "calcs": cfg.get("calcs", ["lastNotNull"]),
                    "fields": "", "values": False,
                },
                "showUnfilled": True,
                "sizing": "auto",
                "valueMode": "color",
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "bargauge",
        }

    def heatmap(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["heatmap"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        scheme = cfg.get("color_scheme", "Spectral")
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "continuous-GrYlRd"},
                    "custom": {
                        "fillOpacity": 80,
                        "hideFrom": {"legend": False, "tooltip": False, "viz": False},
                        "lineWidth": 1,
                    },
                    "mappings": [],
                    "thresholds": {"mode": "absolute", "steps": [{"color": "green", "value": None}]},
                    "unit": cfg.get("unit", "short"),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "calculate": cfg.get("calculate", False),
                "cellGap": cfg.get("cell_gap", 2),
                "cellValues": {"decimals": cfg.get("decimals", 0)},
                "color": {
                    "exponent": 0.5,
                    "fill": "dark-blue",
                    "min": 0,
                    "mode": "scheme",
                    "reverse": False,
                    "scale": cfg.get("color_scale", "exponential"),
                    "scheme": scheme,
                    "steps": 128,
                },
                "exemplars": {"color": "rgba(153,204,255,0.7)"},
                "filterValues": {"le": 1e-9},
                "legend": {"show": True},
                "rowsFrame": {"layout": "auto"},
                "tooltip": {"show": True, "yHistogram": False},
                "yAxis": {
                    "axisPlacement": "left",
                    "reverse": False,
                    "unit": cfg.get("y_unit", "short"),
                },
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "heatmap",
        }

    def histogram(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["histogram"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": cfg.get("color_mode", "palette-classic-by-name")},
                    "custom": {
                        "fillOpacity": cfg.get("fill_opacity", 80),
                        "gradientMode": "none",
                        "hideFrom": {"legend": False, "tooltip": False, "viz": False},
                        "lineWidth": 1,
                    },
                    "mappings": [],
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "short"),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "bucketCount": cfg.get("bucket_count", 30),
                "combine": cfg.get("combine", False),
                "fillOpacity": cfg.get("fill_opacity", 80),
                "gradientMode": "none",
                "legend": {"calcs": [], "displayMode": "list", "placement": "bottom", "showLegend": True},
                "tooltip": {"mode": "multi", "sort": "desc"},
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "histogram",
        }

    def table(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["table"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "thresholds"},
                    "custom": {
                        "align": "auto",
                        "cellOptions": {"type": "auto"},
                        "filterable": cfg.get("filterable", True),
                        "inspect": True,
                    },
                    "mappings": self._value_mappings(cfg),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "short"),
                    "links": self._data_links(cfg),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "cellHeight": "sm",
                "footer": {
                    "countRows": False,
                    "enablePagination": cfg.get("pagination", False),
                    "fields": "",
                    "reducer": ["sum"],
                    "show": False,
                },
                "showHeader": True,
                "sortBy": cfg.get("sort_by", []),
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transformations": cfg.get("transformations", []),
            "transparent": cfg.get("transparent", True),
            "type": "table",
        }

    def piechart(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["piechart"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": cfg.get("color_mode", "palette-classic-by-name")},
                    "mappings": self._value_mappings(cfg),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "short"),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "displayLabels": cfg.get("display_labels", ["percent"]),
                "legend": {
                    "calcs": cfg.get("legend_calcs", []),
                    "displayMode": cfg.get("legend_mode", "list"),
                    "placement": cfg.get("legend_placement", "right"),
                    "showLegend": True,
                },
                "pieType": cfg.get("pie_type", "donut"),
                "reduceOptions": {
                    "calcs": cfg.get("calcs", ["lastNotNull"]),
                    "fields": "", "values": False,
                },
                "tooltip": {"mode": "multi", "sort": "desc"},
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "piechart",
        }

    def state_timeline(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["state-timeline"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "thresholds"},
                    "custom": {
                        "fillOpacity": cfg.get("fill_opacity", 70),
                        "hideFrom": {"legend": False, "tooltip": False, "viz": False},
                        "lineWidth": 0,
                    },
                    "mappings": self._value_mappings(cfg),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "short"),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "alignValue": "center",
                "legend": {"displayMode": "list", "placement": "bottom", "showLegend": True},
                "mergeValues": cfg.get("merge_values", True),
                "rowHeight": cfg.get("row_height", 0.9),
                "showValue": cfg.get("show_value", "auto"),
                "tooltip": {"mode": "multi", "sort": "desc"},
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "state-timeline",
        }

    def status_history(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["status-history"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "thresholds"},
                    "custom": {
                        "fillOpacity": cfg.get("fill_opacity", 70),
                        "hideFrom": {"legend": False, "tooltip": False, "viz": False},
                        "lineWidth": 1,
                    },
                    "mappings": self._value_mappings(cfg),
                    "thresholds": {"mode": "absolute", "steps": self._thresholds(cfg)},
                    "unit": cfg.get("unit", "short"),
                },
                "overrides": self._overrides(cfg),
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "colWidth": 0.9,
                "legend": {"displayMode": "list", "placement": "bottom", "showLegend": True},
                "rowHeight": cfg.get("row_height", 0.9),
                "showValue": cfg.get("show_value", "auto"),
                "tooltip": {"mode": "multi", "sort": "desc"},
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "status-history",
        }

    def text(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["text"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "code": {"language": "plaintext", "showLineNumbers": False, "showMiniMap": False},
                "content": cfg.get("content", ""),
                "mode": cfg.get("mode", "markdown"),
            },
            "pluginVersion": "11.2.0",
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "text",
        }

    def logs(self, cfg, x, y):
        dw, dh = DEFAULT_SIZES["logs"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        return {
            "datasource": self._ds(cfg),
            "description": cfg.get("description", ""),
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "dedupStrategy": cfg.get("dedup", "none"),
                "enableLogDetails": True,
                "prettifyLogMessage": cfg.get("prettify", False),
                "showCommonLabels": cfg.get("show_common_labels", False),
                "showLabels": cfg.get("show_labels", False),
                "showTime": cfg.get("show_time", True),
                "sortOrder": cfg.get("sort_order", "Descending"),
                "wrapLogMessage": cfg.get("wrap", True),
            },
            "pluginVersion": "11.2.0",
            "targets": self._build_targets(cfg),
            "title": cfg.get("title", ""),
            "transparent": cfg.get("transparent", True),
            "type": "logs",
        }

    def comparison(self, cfg, x, y):
        """Build a mixed-datasource comparison panel."""
        dw, dh = DEFAULT_SIZES["comparison"]
        w = cfg.get("width", dw)
        h = cfg.get("height", dh)
        ds_names = cfg.get("datasources", [])
        if len(ds_names) < 2:
            raise ValueError("comparison panel requires at least 2 datasources")

        metric = cfg.get("metric", "up")
        metric_type = cfg.get("metric_type", "gauge")
        mixed_ds = {"type": "datasource", "uid": "-- Mixed --"}

        targets = []
        for i, ds_name in enumerate(ds_names):
            ds = self.config.get_datasource(ds_name)
            if metric_type == "counter":
                expr = f"rate({metric}[5m])"
            else:
                expr = metric
            legend = cfg.get("legend", f"{ds_name}: {{{{instance}}}}")
            if f"{ds_name}" not in legend:
                legend = f"{ds_name}: {legend}"
            targets.append({
                "datasource": ds,
                "editorMode": "code",
                "expr": self.config.resolve_ref(expr),
                "legendFormat": legend,
                "range": True,
                "refId": chr(65 + i),
            })

        return {
            "datasource": mixed_ds,
            "description": cfg.get("description", f"comparison: {metric}"),
            "fieldConfig": {
                "defaults": {
                    "color": {"mode": "palette-classic-by-name"},
                    "custom": {
                        "axisBorderShow": False,
                        "axisCenteredZero": False,
                        "axisColorMode": "text",
                        "axisLabel": "",
                        "axisPlacement": "auto",
                        "barAlignment": 0,
                        "barWidthFactor": 0.6,
                        "drawStyle": "line",
                        "fillOpacity": 8,
                        "gradientMode": "scheme",
                        "hideFrom": {"legend": False, "tooltip": False, "viz": False},
                        "insertNulls": False,
                        "lineInterpolation": "smooth",
                        "lineWidth": 1,
                        "pointSize": 5,
                        "scaleDistribution": {"type": "linear"},
                        "showPoints": "never",
                        "spanNulls": False,
                        "stacking": {"group": "A", "mode": "none"},
                        "thresholdsStyle": {"mode": "off"},
                    },
                    "mappings": [],
                    "thresholds": {"mode": "absolute", "steps": [{"color": "#73BF69", "value": None}]},
                    "unit": cfg.get("unit", "short"),
                },
                "overrides": [],
            },
            "gridPos": {"h": h, "w": w, "x": x, "y": y},
            "id": self.id_gen.next(),
            "options": {
                "legend": {"calcs": [], "displayMode": "list", "placement": "bottom", "showLegend": True},
                "tooltip": {"mode": "multi", "sort": "desc"},
            },
            "pluginVersion": "11.2.0",
            "targets": targets,
            "title": cfg.get("title", f"{metric} comparison"),
            "transparent": cfg.get("transparent", True),
            "type": "timeseries",
        }


# ──────────────────────────────────────────────────────────────────────────────
# METRIC DISCOVERY
# ──────────────────────────────────────────────────────────────────────────────

class MetricDiscovery:
    def __init__(self, config):
        self.config = config
        self._cache = {}

    def _get(self, url, path):
        full = f"{url.rstrip('/')}{path}"
        try:
            resp = urllib.request.urlopen(full, timeout=30)
            body = json.loads(resp.read())
            if body.get("status") != "success":
                print(f"  warning: non-success response from {full}", file=sys.stderr)
                return body.get("data", [])
            return body["data"]
        except (urllib.error.URLError, urllib.error.HTTPError) as e:
            print(f"  error querying {full}: {e}", file=sys.stderr)
            return []

    def fetch_metrics(self, ds_name):
        url = self.config.get_datasource_url(ds_name)
        if not url:
            raise ValueError(f"no URL configured for datasource '{ds_name}'")
        key = f"metrics:{ds_name}"
        if key not in self._cache:
            data = self._get(url, "/api/v1/label/__name__/values")
            self._cache[key] = set(data) if isinstance(data, list) else set()
        return self._cache[key]

    def fetch_metadata(self, ds_name):
        url = self.config.get_datasource_url(ds_name)
        if not url:
            return {}
        key = f"metadata:{ds_name}"
        if key not in self._cache:
            data = self._get(url, "/api/v1/metadata")
            meta = {}
            if isinstance(data, dict):
                for metric, info_list in data.items():
                    if info_list and isinstance(info_list, list):
                        meta[metric] = {
                            "type": info_list[0].get("type", "untyped"),
                            "help": info_list[0].get("help", ""),
                        }
            self._cache[key] = meta
        return self._cache[key]

    def fetch_labels(self, ds_name):
        url = self.config.get_datasource_url(ds_name)
        if not url:
            return []
        data = self._get(url, "/api/v1/labels")
        return data if isinstance(data, list) else []

    def fetch_label_values(self, ds_name, label):
        url = self.config.get_datasource_url(ds_name)
        if not url:
            return []
        data = self._get(url, f"/api/v1/label/{label}/values")
        return data if isinstance(data, list) else []

    def categorize(self, ds_a, ds_b):
        metrics_a = self.fetch_metrics(ds_a)
        metrics_b = self.fetch_metrics(ds_b)
        meta_a = self.fetch_metadata(ds_a)
        meta_b = self.fetch_metadata(ds_b)

        shared = metrics_a & metrics_b
        only_a = metrics_a - metrics_b
        only_b = metrics_b - metrics_a

        def enrich(names, meta_primary, meta_fallback):
            result = {}
            for m in sorted(names):
                info = meta_primary.get(m, meta_fallback.get(m, {}))
                result[m] = {
                    "type": info.get("type", "untyped"),
                    "help": info.get("help", ""),
                }
            return result

        return {
            "shared": enrich(shared, meta_a, meta_b),
            "only_a": enrich(only_a, meta_a, {}),
            "only_b": enrich(only_b, {}, meta_b),
        }

    def filter_metrics(self, metrics, include_patterns=None, exclude_patterns=None):
        include = include_patterns or ["*"]
        exclude = exclude_patterns or []
        filtered = set()
        for m in metrics:
            included = any(fnmatch.fnmatch(m, p) for p in include)
            excluded = any(fnmatch.fnmatch(m, p) for p in exclude)
            if included and not excluded:
                filtered.add(m)
        return filtered

    def group_by_prefix(self, metrics_dict):
        groups = defaultdict(dict)
        for metric, info in metrics_dict.items():
            parts = metric.split("_")
            if len(parts) >= 2:
                prefix = f"{parts[0]}_{parts[1]}"
            else:
                prefix = parts[0]
            groups[prefix][metric] = info
        return dict(sorted(groups.items()))

    @staticmethod
    def suggest_panel_type(metric_type):
        mapping = {
            "counter": "timeseries",
            "gauge": "stat",
            "histogram": "heatmap",
            "summary": "timeseries",
            "untyped": "timeseries",
        }
        return mapping.get(metric_type, "timeseries")

    @staticmethod
    def suggest_query(metric_name, metric_type):
        if metric_type == "counter":
            return f"rate({metric_name}[5m])"
        elif metric_type == "histogram":
            return metric_name
        elif metric_type == "summary":
            return metric_name
        else:
            return metric_name

    def print_discovery(self, sources, include_patterns=None, exclude_patterns=None):
        if len(sources) == 1:
            ds_name = sources[0]
            metrics = self.fetch_metrics(ds_name)
            metrics = self.filter_metrics(metrics, include_patterns, exclude_patterns)
            meta = self.fetch_metadata(ds_name)

            print(f"\n=== Metrics from {ds_name}: {len(metrics)} total ===\n")
            grouped = self.group_by_prefix({m: meta.get(m, {"type": "untyped", "help": ""}) for m in metrics})
            for prefix, items in grouped.items():
                print(f"# {prefix}_* ({len(items)} metrics)")
                for m, info in sorted(items.items()):
                    mtype = info["type"]
                    panel = self.suggest_panel_type(mtype)
                    print(f"  {m:60s} ({mtype:10s}) -> {panel}")
                print()

            self._print_yaml_snippet(grouped, meta, ds_name)

        elif len(sources) == 2:
            cats = self.categorize(sources[0], sources[1])
            cats["shared"] = {k: v for k, v in cats["shared"].items()
                             if k in self.filter_metrics(set(cats["shared"].keys()), include_patterns, exclude_patterns)}
            cats["only_a"] = {k: v for k, v in cats["only_a"].items()
                             if k in self.filter_metrics(set(cats["only_a"].keys()), include_patterns, exclude_patterns)}
            cats["only_b"] = {k: v for k, v in cats["only_b"].items()
                             if k in self.filter_metrics(set(cats["only_b"].keys()), include_patterns, exclude_patterns)}

            print(f"\n=== Metric Comparison ===")
            print(f"  {sources[0]}: {len(cats['only_a']) + len(cats['shared'])} metrics")
            print(f"  {sources[1]}: {len(cats['only_b']) + len(cats['shared'])} metrics")
            print(f"  shared: {len(cats['shared'])}")
            print(f"  {sources[0]} only: {len(cats['only_a'])}")
            print(f"  {sources[1]} only: {len(cats['only_b'])}")

            print(f"\n--- Shared Metrics ({len(cats['shared'])}) ---")
            for m, info in sorted(cats["shared"].items()):
                print(f"  {m:60s} ({info['type']})")

            print(f"\n--- {sources[0]} Only ({len(cats['only_a'])}) ---")
            for m, info in sorted(cats["only_a"].items()):
                print(f"  {m:60s} ({info['type']})")

            print(f"\n--- {sources[1]} Only ({len(cats['only_b'])}) ---")
            for m, info in sorted(cats["only_b"].items()):
                print(f"  {m:60s} ({info['type']})")

            self._print_comparison_yaml(cats, sources)

    def _print_yaml_snippet(self, grouped, meta, ds_name):
        print("\n# --- suggested YAML config snippet ---\n")
        print("dashboards:")
        print("  discovered:")
        print(f"    uid: discovered-{ds_name}")
        print(f"    title: discovered metrics ({ds_name})")
        print(f"    filename: discovered-{ds_name}.json")
        print(f"    tags: [discovered]")
        print(f"    variables: []")
        print(f"    sections:")
        for prefix, items in grouped.items():
            print(f"      - title: \"{prefix}\"")
            print(f"        panels:")
            for m, info in sorted(items.items()):
                mtype = info.get("type", "untyped")
                panel = self.suggest_panel_type(mtype)
                query = self.suggest_query(m, mtype)
                print(f"          - type: {panel}")
                print(f"            title: \"{m}\"")
                print(f"            query: '{query}'")

    def _print_comparison_yaml(self, cats, sources):
        print("\n# --- suggested comparison YAML snippet ---\n")
        print("dashboards:")
        print("  comparison:")
        print(f"    uid: metric-comparison")
        print(f"    title: metric comparison")
        print(f"    filename: metric-comparison.json")
        print(f"    tags: [comparison]")
        print(f"    variables: []")
        print(f"    sections:")

        if cats["shared"]:
            print(f"      - title: \"shared metrics\"")
            print(f"        panels:")
            for m in sorted(cats["shared"].keys()):
                info = cats["shared"][m]
                print(f"          - type: comparison")
                print(f"            title: \"{m}\"")
                print(f"            metric: \"{m}\"")
                print(f"            metric_type: \"{info['type']}\"")
                print(f"            datasources: [{sources[0]}, {sources[1]}]")

        if cats["only_a"]:
            print(f"      - title: \"{sources[0]} only\"")
            print(f"        panels:")
            for m in sorted(cats["only_a"].keys()):
                info = cats["only_a"][m]
                panel = self.suggest_panel_type(info["type"])
                query = self.suggest_query(m, info["type"])
                print(f"          - type: {panel}")
                print(f"            title: \"{m}\"")
                print(f"            query: '{query}'")
                print(f"            datasource: {sources[0]}")

        if cats["only_b"]:
            print(f"      - title: \"{sources[1]} only\"")
            print(f"        panels:")
            for m in sorted(cats["only_b"].keys()):
                info = cats["only_b"][m]
                panel = self.suggest_panel_type(info["type"])
                query = self.suggest_query(m, info["type"])
                print(f"          - type: {panel}")
                print(f"            title: \"{m}\"")
                print(f"            query: '{query}'")
                print(f"            datasource: {sources[1]}")

    def generate_discovery_sections(self, sources, include_patterns=None, exclude_patterns=None):
        """Generate dashboard sections from discovered metrics (for auto-generation mode)."""
        sections = []

        if len(sources) == 1:
            ds_name = sources[0]
            metrics = self.fetch_metrics(ds_name)
            metrics = self.filter_metrics(metrics, include_patterns, exclude_patterns)
            meta = self.fetch_metadata(ds_name)
            grouped = self.group_by_prefix({m: meta.get(m, {"type": "untyped"}) for m in metrics})

            for prefix, items in grouped.items():
                panels = []
                for m, info in sorted(items.items()):
                    mtype = info.get("type", "untyped")
                    panels.append({
                        "type": self.suggest_panel_type(mtype),
                        "title": m,
                        "query": self.suggest_query(m, mtype),
                        "datasource": ds_name,
                    })
                sections.append({"title": prefix, "panels": panels})

        elif len(sources) == 2:
            cats = self.categorize(sources[0], sources[1])
            cats["shared"] = {k: v for k, v in cats["shared"].items()
                             if k in self.filter_metrics(set(cats["shared"].keys()), include_patterns, exclude_patterns)}
            cats["only_a"] = {k: v for k, v in cats["only_a"].items()
                             if k in self.filter_metrics(set(cats["only_a"].keys()), include_patterns, exclude_patterns)}
            cats["only_b"] = {k: v for k, v in cats["only_b"].items()
                             if k in self.filter_metrics(set(cats["only_b"].keys()), include_patterns, exclude_patterns)}

            if cats["shared"]:
                panels = []
                for m in sorted(cats["shared"].keys()):
                    info = cats["shared"][m]
                    panels.append({
                        "type": "comparison",
                        "title": m,
                        "metric": m,
                        "metric_type": info["type"],
                        "datasources": list(sources),
                    })
                sections.append({"title": "shared metrics", "panels": panels})

            if cats["only_a"]:
                panels = []
                for m in sorted(cats["only_a"].keys()):
                    info = cats["only_a"][m]
                    panels.append({
                        "type": self.suggest_panel_type(info["type"]),
                        "title": m,
                        "query": self.suggest_query(m, info["type"]),
                        "datasource": sources[0],
                    })
                sections.append({"title": f"{sources[0]} only", "panels": panels})

            if cats["only_b"]:
                panels = []
                for m in sorted(cats["only_b"].keys()):
                    info = cats["only_b"][m]
                    panels.append({
                        "type": self.suggest_panel_type(info["type"]),
                        "title": m,
                        "query": self.suggest_query(m, info["type"]),
                        "datasource": sources[1],
                    })
                sections.append({"title": f"{sources[1]} only", "panels": panels})

        return sections


# ──────────────────────────────────────────────────────────────────────────────
# DASHBOARD BUILDER
# ──────────────────────────────────────────────────────────────────────────────

class DashboardBuilder:
    def __init__(self, config, panel_factory, layout_engine):
        self.config = config
        self.factory = panel_factory
        self.layout = layout_engine

    def build_navigation_links(self, all_dashboards):
        links = []
        for db_cfg in all_dashboards.values():
            links.append({
                "title": db_cfg.get("title", ""),
                "type": "link",
                "url": f"/d/{db_cfg['uid']}",
                "icon": db_cfg.get("icon", "apps"),
                "targetBlank": False,
                "keepTime": True,
                "includeVars": True,
                "tooltip": db_cfg.get("description", ""),
            })
        return links

    def build_variable(self, name):
        v = self.config.get_variable_def(name)
        if not v:
            raise ValueError(f"variable '{name}' not defined in config")

        vtype = v.get("type", "query")
        query = self.config.resolve_ref(v.get("query", ""))
        multi = v.get("multi", False)
        include_all = v.get("include_all", False)
        refresh = v.get("refresh", 2)
        sort = v.get("sort", 1)
        label = v.get("label", name)
        hide = v.get("hide", 0)
        regex = v.get("regex", "")
        all_value = v.get("all_value", "")

        ds = self.config.get_default_datasource()
        if v.get("datasource"):
            ds = self.config.get_datasource(v["datasource"])

        current = {"selected": True, "text": "All", "value": "$__all"}
        if not include_all:
            default = v.get("default", {})
            current = {
                "selected": True,
                "text": default.get("text", ""),
                "value": default.get("value", ""),
            }

        var = {
            "current": current,
            "datasource": ds,
            "definition": query,
            "hide": hide,
            "includeAll": include_all,
            "label": label,
            "multi": multi,
            "name": name,
            "options": [],
            "query": {"query": query, "refId": "StandardVariableQuery"},
            "refresh": refresh,
            "regex": regex,
            "skipUrlSync": False,
            "sort": sort,
            "type": vtype,
        }

        if all_value:
            var["allValue"] = all_value

        if vtype == "custom":
            var["query"] = v.get("values", "")
            del var["datasource"]
            del var["definition"]

        if vtype == "datasource":
            var["query"] = v.get("ds_type", "prometheus")
            del var["definition"]
            del var["datasource"]

        if vtype == "interval":
            var["query"] = v.get("values", "1m,5m,15m,30m,1h,6h,12h,1d")
            var["auto"] = v.get("auto", False)
            var["auto_count"] = v.get("auto_count", 10)
            var["auto_min"] = v.get("auto_min", "10s")
            del var["datasource"]
            del var["definition"]

        return var

    def build_variables(self, var_names):
        return [self.build_variable(name) for name in var_names]

    def build_section(self, section_cfg):
        panels = []
        title = section_cfg.get("title", "")
        collapsed = section_cfg.get("collapsed", False)
        repeat = section_cfg.get("repeat")

        if collapsed:
            # collapsed row: panels are nested inside the row
            inner_layout = LayoutEngine()
            inner_panels = []
            for pcfg in section_cfg.get("panels", []):
                ptype = pcfg["type"]
                dw, dh = DEFAULT_SIZES.get(ptype, (6, 4))
                w = pcfg.get("width", dw)
                h = pcfg.get("height", dh)

                if pcfg.get("x") is not None and pcfg.get("y") is not None:
                    px, py = pcfg["x"], pcfg["y"]
                else:
                    px, py = inner_layout.place(w, h)

                if ptype == "comparison":
                    inner_panels.append(self.factory.comparison(pcfg, px, py))
                else:
                    inner_panels.append(self.factory.from_config(pcfg, px, py))

            row_y = self.layout.add_row()
            panels.append(self.factory.row(title, row_y, collapsed=True, panels=inner_panels, repeat=repeat))
        else:
            # open row: row header + panels sequentially
            row_y = self.layout.add_row()
            panels.append(self.factory.row(title, row_y, repeat=repeat))

            for pcfg in section_cfg.get("panels", []):
                ptype = pcfg["type"]
                dw, dh = DEFAULT_SIZES.get(ptype, (6, 4))
                w = pcfg.get("width", dw)
                h = pcfg.get("height", dh)

                if pcfg.get("x") is not None and pcfg.get("y") is not None:
                    px, py = pcfg["x"], pcfg["y"]
                else:
                    px, py = self.layout.place(w, h)

                if ptype == "comparison":
                    panels.append(self.factory.comparison(pcfg, px, py))
                else:
                    panels.append(self.factory.from_config(pcfg, px, py))

            self.layout.finish_section()

        return panels

    def build(self, db_cfg, nav_links, discovery_sections=None):
        self.factory.id_gen.reset()
        self.layout.reset()

        gen = self.config.get_generator()
        var_names = db_cfg.get("variables", [])
        variables = self.build_variables(var_names)

        all_panels = []
        for section in db_cfg.get("sections", []):
            all_panels.extend(self.build_section(section))

        if discovery_sections:
            for section in discovery_sections:
                all_panels.extend(self.build_section(section))

        return {
            "annotations": {
                "list": [{
                    "builtIn": 1,
                    "datasource": {"type": "grafana", "uid": "-- Grafana --"},
                    "enable": True,
                    "hide": True,
                    "iconColor": "rgba(0, 211, 255, 1)",
                    "name": "Annotations & Alerts",
                    "type": "dashboard",
                }]
            },
            "description": db_cfg.get("description", ""),
            "editable": gen.get("editable", True),
            "fiscalYearStartMonth": 0,
            "graphTooltip": gen.get("graph_tooltip", 1),
            "id": None,
            "links": nav_links,
            "liveNow": gen.get("live_now", True),
            "panels": all_panels,
            "refresh": gen.get("refresh", "30s"),
            "schemaVersion": gen.get("schema_version", 39),
            "tags": db_cfg.get("tags", []),
            "templating": {"list": variables},
            "time": gen.get("time_range", {"from": "now-30m", "to": "now"}),
            "timepicker": {
                "refresh_intervals": ["5s", "10s", "30s", "1m", "5m", "15m", "30m"],
            },
            "timezone": gen.get("timezone", ""),
            "title": db_cfg.get("title", ""),
            "uid": db_cfg["uid"],
            "version": 1,
        }


# ──────────────────────────────────────────────────────────────────────────────
# GRAFANA API PUSH
# ──────────────────────────────────────────────────────────────────────────────

def push_to_grafana(dashboard, grafana_url, auth_user=None, auth_pass=None, token=None):
    payload = json.dumps({
        "dashboard": dashboard,
        "overwrite": True,
        "message": "updated by grafana-dashboard-generator",
    }).encode()

    headers = {"Content-Type": "application/json"}
    if token:
        headers["Authorization"] = f"Bearer {token}"
    elif auth_user and auth_pass:
        creds = base64.b64encode(f"{auth_user}:{auth_pass}".encode()).decode()
        headers["Authorization"] = f"Basic {creds}"

    url = f"{grafana_url.rstrip('/')}/api/dashboards/db"
    req = urllib.request.Request(url, data=payload, headers=headers, method="POST")
    try:
        resp = urllib.request.urlopen(req, timeout=30)
        result = json.loads(resp.read())
        status = result.get("status", "unknown")
        uid = result.get("uid", dashboard.get("uid", "?"))
        print(f"  pushed {uid}: {status}")
        return True
    except (urllib.error.URLError, urllib.error.HTTPError) as e:
        print(f"  error pushing dashboard: {e}", file=sys.stderr)
        return False


# ──────────────────────────────────────────────────────────────────────────────
# OUTPUT
# ──────────────────────────────────────────────────────────────────────────────

def write_dashboard(dashboard, filepath, dry_run=False):
    data = json.dumps(dashboard, indent=2) + "\n"
    size = len(data.encode("utf-8"))
    panel_count = len(dashboard.get("panels", []))
    filename = os.path.basename(filepath)

    if size > 750_000:
        print(f"  WARNING: {filename} is {size:,} bytes (>750KB ConfigMap limit)")

    if not dry_run:
        with open(filepath, "w") as f:
            f.write(data)

    print(f"  {filename}: {panel_count} panels, {size:,} bytes")
    return size


# ──────────────────────────────────────────────────────────────────────────────
# MAIN
# ──────────────────────────────────────────────────────────────────────────────

def parse_args():
    p = argparse.ArgumentParser(description="General-purpose Grafana dashboard generator")
    p.add_argument("--config", required=True, help="path to YAML config file")
    p.add_argument("--profile", help="generate only dashboards in named profile")
    p.add_argument("--output-dir", help="override output directory")
    p.add_argument("--prometheus-url", help="prometheus URL for metric discovery")
    p.add_argument("--grafana-url", help="grafana URL for --push mode")
    p.add_argument("--grafana-user", help="grafana basic auth user (for --push)")
    p.add_argument("--grafana-pass", help="grafana basic auth password (for --push)")
    p.add_argument("--grafana-token", help="grafana API token (for --push)")
    p.add_argument("--discover-print", action="store_true",
                   help="query prometheus and print suggested YAML config")
    p.add_argument("--dry-run", action="store_true", help="generate to memory only")
    p.add_argument("--verbose", action="store_true", help="print panel details")
    p.add_argument("--push", action="store_true", help="push dashboards to Grafana API")
    return p.parse_args()


def main():
    args = parse_args()

    cli_args = {}
    if args.prometheus_url:
        cli_args["prometheus_url"] = args.prometheus_url

    config = Config.load(args.config, cli_args)
    gen = config.get_generator()

    # determine output directory
    output_dir = args.output_dir or gen.get("output_dir", ".")
    if not os.path.isabs(output_dir):
        output_dir = os.path.join(os.path.dirname(os.path.abspath(args.config)), output_dir)
    os.makedirs(output_dir, exist_ok=True)

    # discovery mode
    if args.discover_print:
        discovery_cfg = config.get_discovery()
        sources = discovery_cfg.get("sources", [])
        if not sources:
            ds_map = config.data.get("datasources", {})
            sources = list(ds_map.keys())
        if not sources:
            print("error: no datasources configured for discovery", file=sys.stderr)
            sys.exit(1)

        disc = MetricDiscovery(config)
        disc.print_discovery(
            sources,
            include_patterns=discovery_cfg.get("include_patterns"),
            exclude_patterns=discovery_cfg.get("exclude_patterns"),
        )
        return

    # get dashboards to generate
    dashboards = config.get_dashboards(args.profile)
    if not dashboards:
        print("error: no dashboards defined in config", file=sys.stderr)
        sys.exit(1)

    # build components
    id_gen = IdGenerator()
    panel_factory = PanelFactory(config, id_gen)
    layout_engine = LayoutEngine()
    builder = DashboardBuilder(config, panel_factory, layout_engine)

    # build navigation links from all dashboards
    nav_links = builder.build_navigation_links(dashboards)

    # auto-discovery sections if enabled
    discovery_sections = None
    discovery_cfg = config.get_discovery()
    if discovery_cfg.get("enabled"):
        sources = discovery_cfg.get("sources", [])
        if sources:
            disc = MetricDiscovery(config)
            discovery_sections = disc.generate_discovery_sections(
                sources,
                include_patterns=discovery_cfg.get("include_patterns"),
                exclude_patterns=discovery_cfg.get("exclude_patterns"),
            )

    # generate dashboards
    total_size = 0
    total_panels = 0
    print("grafana dashboard generator:")

    for name, db_cfg in dashboards.items():
        dashboard = builder.build(db_cfg, nav_links, discovery_sections)
        filename = db_cfg.get("filename", f"{name}.json")
        filepath = os.path.join(output_dir, filename)

        size = write_dashboard(dashboard, filepath, args.dry_run)
        total_size += size
        total_panels += len(dashboard.get("panels", []))

        if args.verbose:
            for panel in dashboard.get("panels", []):
                ptype = panel.get("type", "?")
                ptitle = panel.get("title", "?")
                print(f"    [{ptype}] {ptitle}")

        if args.push and args.grafana_url:
            push_to_grafana(
                dashboard, args.grafana_url,
                auth_user=args.grafana_user,
                auth_pass=args.grafana_pass,
                token=args.grafana_token,
            )

    print(f"\n  total: {len(dashboards)} dashboards, {total_panels} panels, {total_size:,} bytes")


if __name__ == "__main__":
    main()

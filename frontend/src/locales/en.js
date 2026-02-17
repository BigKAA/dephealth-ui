// English translation dictionary
export default {
  // Header
  'header.title': 'dephealth-ui',

  // Toolbar buttons
  'toolbar.alerts': 'Active alerts',
  'toolbar.filter': 'Toggle filters',
  'toolbar.refresh': 'Refresh now',
  'toolbar.autoRefresh': 'Toggle auto-refresh',
  'toolbar.fit': 'Fit graph to screen',
  'toolbar.theme': 'Toggle dark/light theme',
  'toolbar.language': 'Language',
  'toolbar.logout': 'Logout',

  // Connection banner
  'banner.message': 'Connection lost. Retrying in {seconds}s...',
  'banner.retry': 'Retry now',

  // Filter panel
  'filter.namespace': 'Namespace',
  'filter.type': 'Type',
  'filter.state': 'State',
  'filter.service': 'Service',
  'filter.reset': 'Reset',
  'filter.allNamespaces': 'All namespaces',
  'filter.allTypes': 'All types',
  'filter.allServices': 'All services',
  'filter.allStatuses': 'All statuses',
  'filter.status': 'Status',

  // Graph toolbar
  'graphToolbar.zoomIn': 'Zoom in',
  'graphToolbar.zoomOut': 'Zoom out',
  'graphToolbar.fit': 'Fit to screen',
  'graphToolbar.search': 'Search nodes',
  'graphToolbar.layout': 'Toggle layout direction',
  'graphToolbar.export': 'Export as PNG',
  'graphToolbar.fullscreen': 'Toggle fullscreen',
  'graphToolbar.legend': 'Toggle legend',
  'graphToolbar.groupByNs': 'Group by namespace',
  'graphToolbar.ungroupNs': 'Disable namespace grouping',
  'graphToolbar.collapseAll': 'Collapse all namespaces',
  'graphToolbar.expandAll': 'Expand all namespaces',
  'graphToolbar.nsLegend': 'Toggle namespace legend',
  'graphToolbar.connLegend': 'Toggle connection legend',

  // Search panel
  'search.placeholder': 'Search nodes...',
  'search.close': 'Close search',
  'search.noMatches': 'No matches',

  // Legend
  'legend.title': 'Legend',
  'legend.close': 'Close legend',
  'legend.nodeStates': 'Node States',
  'legend.edgeStates': 'Edge States',
  'legend.alerts': 'Alerts',
  'legend.ok': 'OK',
  'legend.degraded': 'Degraded',
  'legend.down': 'Down',
  'legend.unknown': 'Unknown',
  'legend.criticalAlert': 'Critical alert',
  'legend.cascadeWarning': 'Cascade warning',
  'legend.thickerBorder': 'Thicker border',
  'legend.connectionStatuses': 'Connection Statuses',
  'legend.status.timeout': 'Timeout',
  'legend.status.connectionError': 'Connection Error',
  'legend.status.dnsError': 'DNS Error',
  'legend.status.authError': 'Auth Error',
  'legend.status.tlsError': 'TLS Error',
  'legend.status.unhealthy': 'Unhealthy',
  'legend.status.error': 'Error',

  // Empty state
  'empty.title': 'No services found',
  'empty.description': 'Check that Prometheus is configured with topologymetrics exporter.',

  // Error overlay
  'error.title': 'Connection Error',
  'error.retry': 'Retry',

  // Status bar
  'status.updated': 'Updated {time} | {nodes} nodes, {edges} edges',
  'status.alerts': 'Alerts: {details}',
  'status.critical': '{count} critical',
  'status.warning': '{count} warning',
  'status.partialData': 'Partial data',
  'status.filtered': 'Filtered',
  'status.loading': 'Loading...',
  'status.connectionStatus': 'Connection status',

  // Health stats
  'state.ok': 'OK',
  'state.degraded': 'Degraded',
  'state.down': 'Down',
  'state.unknown': 'Unknown',
  'state.unknown.detail': 'Metrics disappeared',
  'state.warning': 'Warning',

  // Toast messages
  'toast.connectionRestored': 'Connection restored',
  'toast.connectionLost': 'Connection lost: {error}',
  'toast.dataSourceError': 'Data source error: {error}',
  'toast.exportedPNG': 'Graph exported as PNG',
  'toast.close': 'Close',

  // Sidebar
  'sidebar.close': 'Close',
  'sidebar.state': 'State',
  'sidebar.namespace': 'Namespace',
  'sidebar.type': 'Type',
  'sidebar.host': 'Host',
  'sidebar.port': 'Port',
  'sidebar.activeAlerts': 'Active Alerts',
  'sidebar.activeAlertsCount': 'Active Alerts ({count})',
  'sidebar.connectedEdges': 'Connected Edges ({count})',
  'sidebar.outgoingEdges': 'Outgoing ({count})',
  'sidebar.incomingEdges': 'Incoming ({count})',
  'sidebar.openGrafana': 'Open in Grafana',
  'sidebar.grafanaDashboards': 'Grafana Dashboards',
  'sidebar.grafana.serviceList': 'Service List',
  'sidebar.grafana.servicesStatus': 'Services Status',
  'sidebar.grafana.linksStatus': 'Links Status',
  'sidebar.grafana.cascadeOverview': 'Cascade Overview',
  'sidebar.grafana.rootCause': 'Root Cause Analyzer',
  'sidebar.grafana.serviceStatus': 'Service Status',
  'sidebar.grafana.linkStatus': 'Link Status',
  'sidebar.grafana.connectionDiagnostics': 'Connection Diagnostics',
  'sidebar.instances': 'Instances',
  'sidebar.instancesCount': 'Instances ({count})',
  'sidebar.instanceCol': 'Instance',
  'sidebar.podCol': 'Pod',
  'sidebar.loadingInstances': 'Loading...',
  'sidebar.noInstances': 'No instances found',
  'sidebar.failedInstances': 'Failed to load instances',

  // Edge sidebar
  'sidebar.edge.source': 'Source',
  'sidebar.edge.target': 'Target',
  'sidebar.edge.type': 'Type',
  'sidebar.edge.latency': 'Latency',
  'sidebar.edge.status': 'Status',
  'sidebar.edge.detail': 'Detail',
  'sidebar.edge.critical': 'Critical',
  'sidebar.edge.criticalYes': 'Yes',
  'sidebar.edge.criticalNo': 'No',
  'sidebar.edge.connectedNodes': 'Connected Nodes',
  'sidebar.edge.goToNode': 'Go to node',
  'sidebar.edge.goToEdge': 'Go to edge',
  'sidebar.depStatusSummary': 'Dependency Statuses',

  // Alert drawer
  'alerts.title': 'Active Alerts',
  'alerts.close': 'Close',
  'alerts.empty': 'No active alerts',
  'alerts.unknownAlert': 'Unknown alert',
  'alerts.service': 'Service: {name}',
  'alerts.dependency': 'Dependency: {name}',

  // Tooltip
  'tooltip.state': 'State:',
  'tooltip.type': 'Type:',
  'tooltip.namespace': 'Namespace:',
  'tooltip.alerts': 'Alerts:',
  'tooltip.latency': 'Latency:',
  'tooltip.critical': 'Critical:',
  'tooltip.yes': 'Yes',
  'tooltip.status': 'Status:',
  'tooltip.detail': 'Detail:',
  'tooltip.cascadeWarning': 'Cascade warning:',
  'tooltip.cascadeSource': '↳ {service}',

  // Shortcuts
  'shortcuts.title': 'Keyboard Shortcuts',
  'shortcuts.refresh': 'Refresh graph',
  'shortcuts.fit': 'Fit graph to screen',
  'shortcuts.zoomIn': 'Zoom in',
  'shortcuts.zoomOut': 'Zoom out',
  'shortcuts.search': 'Open search',
  'shortcuts.searchAlt': 'Open search (alternative)',
  'shortcuts.layout': 'Toggle layout direction (TB/LR)',
  'shortcuts.export': 'Export graph as PNG',
  'shortcuts.closeAll': 'Close all panels',
  'shortcuts.help': 'Show this help',

  // Namespace legend
  'namespaceLegend.title': 'Namespaces',
  'namespaceLegend.close': 'Close',
  'namespaceLegend.toggle': 'Toggle namespace legend',

  // Connection legend
  'connLegend.title': 'Connection Statuses',
  'connLegend.close': 'Close',
  'connLegend.color': 'Color',
  'connLegend.abbr': 'Abbr',
  'connLegend.meaning': 'Meaning',

  // Sidebar (collapsed namespace)
  'sidebar.collapsed.services': 'Services ({count})',
  'sidebar.collapsed.worstState': 'Worst State',
  'sidebar.collapsed.totalAlerts': 'Total Alerts',
  'sidebar.collapsed.expand': 'Expand namespace',

  // Context menu
  'contextMenu.rootCauseAnalysis': 'Root Cause Analysis',
  'contextMenu.openInGrafana': 'Open in Grafana',
  'contextMenu.showDetails': 'Show Details',
  'contextMenu.copyGrafanaUrl': 'Copy Grafana URL',
  'contextMenu.urlCopied': 'Grafana URL copied',
  'contextMenu.expandNamespace': 'Expand Namespace',
  'contextMenu.copyNamespaceName': 'Copy Namespace Name',
  'contextMenu.namespaceCopied': 'Namespace name copied',

  // Timeline / History mode
  'toolbar.history': 'History mode',
  'timeline.title': 'Timeline',
  'timeline.live': 'Live',
  'timeline.apply': 'Apply',
  'timeline.historyBanner': 'Viewing historical data: {time}',
  'status.viewing': 'Viewing {time} | {nodes} nodes, {edges} edges',
  'timeline.noData': 'No status changes in this range',
  'timeline.eventsError': 'Failed to load timeline events',
  'timeline.copyUrl': 'Copy URL',
  'timeline.urlCopied': 'URL copied to clipboard',

  // Time ago
  'time.secondsAgo': '{value}s ago',
  'time.minutesAgo': '{value}m ago',
  'time.hoursAgo': '{value}h ago',
  'time.daysAgo': '{value}d ago',
};

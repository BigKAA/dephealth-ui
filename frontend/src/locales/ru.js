// Russian translation dictionary
export default {
  // Header
  'header.title': 'dephealth-ui',

  // Toolbar buttons
  'toolbar.alerts': 'Активные алерты',
  'toolbar.filter': 'Фильтры',
  'toolbar.refresh': 'Обновить',
  'toolbar.autoRefresh': 'Автообновление',
  'toolbar.fit': 'Вписать граф в экран',
  'toolbar.theme': 'Сменить тему',
  'toolbar.language': 'Язык',
  'toolbar.logout': 'Выйти',

  // Connection banner
  'banner.message': 'Соединение потеряно. Повтор через {seconds}с...',
  'banner.retry': 'Повторить',

  // Filter panel
  'filter.namespace': 'Namespace',
  'filter.type': 'Тип',
  'filter.state': 'Состояние',
  'filter.service': 'Сервис',
  'filter.reset': 'Сбросить',
  'filter.allNamespaces': 'Все namespace',
  'filter.allTypes': 'Все типы',
  'filter.allServices': 'Все сервисы',

  // Graph toolbar
  'graphToolbar.zoomIn': 'Приблизить',
  'graphToolbar.zoomOut': 'Отдалить',
  'graphToolbar.fit': 'Вписать в экран',
  'graphToolbar.search': 'Поиск',
  'graphToolbar.layout': 'Направление графа',
  'graphToolbar.export': 'Экспорт PNG',
  'graphToolbar.fullscreen': 'Полный экран',
  'graphToolbar.legend': 'Легенда',
  'graphToolbar.nsLegend': 'Легенда namespace',

  // Search panel
  'search.placeholder': 'Поиск...',
  'search.close': 'Закрыть поиск',
  'search.noMatches': 'Не найдено',

  // Legend
  'legend.title': 'Легенда',
  'legend.close': 'Закрыть',
  'legend.nodeStates': 'Состояния узлов',
  'legend.edgeStates': 'Состояния связей',
  'legend.alerts': 'Алерты',
  'legend.ok': 'OK',
  'legend.degraded': 'Деградация',
  'legend.down': 'Недоступен',
  'legend.unknown': 'Неизвестно',
  'legend.criticalAlert': 'Критический алерт',
  'legend.thickerBorder': 'Утолщённая рамка',

  // Empty state
  'empty.title': 'Сервисы не найдены',
  'empty.description': 'Проверьте, что Prometheus настроен с экспортёром topologymetrics.',

  // Error overlay
  'error.title': 'Ошибка соединения',
  'error.retry': 'Повторить',

  // Status bar
  'status.updated': 'Обновлено {time} | {nodes} узлов, {edges} связей',
  'status.alerts': 'Алерты: {details}',
  'status.critical': '{count} крит.',
  'status.warning': '{count} предупр.',
  'status.partialData': 'Неполные данные',
  'status.filtered': 'Фильтр',
  'status.loading': 'Загрузка...',
  'status.connectionStatus': 'Статус соединения',

  // Health stats
  'state.ok': 'OK',
  'state.degraded': 'Деградация',
  'state.down': 'Недоступен',
  'state.unknown': 'Неизвестно',
  'state.unknown.detail': 'Метрики пропали',

  // Toast messages
  'toast.connectionRestored': 'Соединение восстановлено',
  'toast.connectionLost': 'Соединение потеряно: {error}',
  'toast.dataSourceError': 'Ошибка источника данных: {error}',
  'toast.exportedPNG': 'Граф экспортирован в PNG',
  'toast.close': 'Закрыть',

  // Sidebar
  'sidebar.close': 'Закрыть',
  'sidebar.state': 'Состояние',
  'sidebar.namespace': 'Namespace',
  'sidebar.type': 'Тип',
  'sidebar.host': 'Хост',
  'sidebar.port': 'Порт',
  'sidebar.activeAlerts': 'Активные алерты',
  'sidebar.activeAlertsCount': 'Активные алерты ({count})',
  'sidebar.connectedEdges': 'Связи ({count})',
  'sidebar.outgoingEdges': 'Исходящие ({count})',
  'sidebar.incomingEdges': 'Входящие ({count})',
  'sidebar.openGrafana': 'Открыть в Grafana',
  'sidebar.grafanaDashboards': 'Дашборды Grafana',
  'sidebar.grafana.serviceList': 'Список сервисов',
  'sidebar.grafana.servicesStatus': 'Состояние сервисов',
  'sidebar.grafana.linksStatus': 'Состояние связей',
  'sidebar.grafana.serviceStatus': 'Статус сервиса',
  'sidebar.grafana.linkStatus': 'Статус связи',
  'sidebar.instances': 'Экземпляры',
  'sidebar.instancesCount': 'Экземпляры ({count})',
  'sidebar.instanceCol': 'Экземпляр',
  'sidebar.podCol': 'Pod',
  'sidebar.loadingInstances': 'Загрузка...',
  'sidebar.noInstances': 'Экземпляры не найдены',
  'sidebar.failedInstances': 'Не удалось загрузить экземпляры',

  // Alert drawer
  'alerts.title': 'Активные алерты',
  'alerts.close': 'Закрыть',
  'alerts.empty': 'Нет активных алертов',
  'alerts.unknownAlert': 'Неизвестный алерт',
  'alerts.service': 'Сервис: {name}',
  'alerts.dependency': 'Зависимость: {name}',

  // Tooltip
  'tooltip.state': 'Состояние:',
  'tooltip.type': 'Тип:',
  'tooltip.namespace': 'Namespace:',
  'tooltip.alerts': 'Алерты:',
  'tooltip.latency': 'Задержка:',
  'tooltip.critical': 'Критический:',
  'tooltip.yes': 'Да',

  // Shortcuts
  'shortcuts.title': 'Горячие клавиши',
  'shortcuts.refresh': 'Обновить граф',
  'shortcuts.fit': 'Вписать граф в экран',
  'shortcuts.zoomIn': 'Приблизить',
  'shortcuts.zoomOut': 'Отдалить',
  'shortcuts.search': 'Открыть поиск',
  'shortcuts.searchAlt': 'Открыть поиск (альтернатива)',
  'shortcuts.layout': 'Направление графа (TB/LR)',
  'shortcuts.export': 'Экспорт графа в PNG',
  'shortcuts.closeAll': 'Закрыть все панели',
  'shortcuts.help': 'Показать подсказки',

  // Namespace legend
  'namespaceLegend.title': 'Namespace',
  'namespaceLegend.close': 'Закрыть',
  'namespaceLegend.toggle': 'Легенда namespace',

  // Context menu
  'contextMenu.openInGrafana': 'Открыть в Grafana',
  'contextMenu.showDetails': 'Подробности',
  'contextMenu.copyGrafanaUrl': 'Копировать URL Grafana',
  'contextMenu.urlCopied': 'URL Grafana скопирован',

  // Time ago
  'time.secondsAgo': '{value}с назад',
  'time.minutesAgo': '{value}м назад',
  'time.hoursAgo': '{value}ч назад',
  'time.daysAgo': '{value}д назад',
};

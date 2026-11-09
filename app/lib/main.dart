import 'dart:async';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:media_kit/media_kit.dart';
import 'package:provider/provider.dart';
import 'package:window_manager/window_manager.dart';

import 'app.dart';
import 'core/activity_tracker.dart';
import 'core/app_state.dart';
import 'core/deep_links.dart';
import 'core/background_work.dart';
import 'core/daemon_service.dart';
import 'core/plugins.dart';
import 'core/plugin_navigation.dart';
import 'core/ui_slots.dart';
import 'features/plugins/plugin_contributions.dart';
import 'features/plugins/plugin_web_page.dart';
import 'core/profile.dart';
import 'features/assistant/assistant_controller.dart';
import 'features/assistant/task_inbox.dart';
import 'features/schedule/schedule_store.dart';
import 'features/schedule/schedule_widgets.dart';
import 'theme/prefs.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  MediaKit.ensureInitialized();
  final desktop = Platform.isLinux || Platform.isWindows || Platform.isMacOS;
  if (desktop) {
    await windowManager.ensureInitialized();
    await windowManager.waitUntilReadyToShow(
      const WindowOptions(size: Size(1320, 860), minimumSize: Size(380, 560), title: 'Coursive', center: true),
      () async {
        await windowManager.show();
        await windowManager.focus();
      },
    );
  }
  final prefs = await UiPrefs.load();
  final profile = await UserProfile.load();
  final daemon = DaemonService();
  unawaited(daemon.start());
  final app = AppState(daemon);
  unawaited(app.init());
  ActivityTracker.instance = ActivityTracker(app);
  if (Platform.isAndroid) BackgroundWork(app);
  final screen = ScreenContext();
  final players = PlayerHub();
  final plugins = PluginStore(app);
  final pluginPages = PluginNavigationRegistry();
  final pluginSlots = UiSlotRegistry();
  PluginContributionBinder(store: plugins, pages: pluginPages, slots: pluginSlots);

  runApp(MultiProvider(
    providers: [
      ChangeNotifierProvider.value(value: prefs),
      ChangeNotifierProvider.value(value: profile),
      ChangeNotifierProvider.value(value: daemon),
      ChangeNotifierProvider.value(value: app),
      ChangeNotifierProvider.value(value: screen),
      ChangeNotifierProvider.value(value: players),
      ChangeNotifierProvider(create: (_) => AssistantController(app, screen, players)),
      ChangeNotifierProvider(create: (_) => ScheduleStore(app)),
      ChangeNotifierProvider(create: (_) => TaskInbox(app)),
      ChangeNotifierProvider.value(value: plugins),
      ChangeNotifierProvider.value(value: pluginPages),
      ChangeNotifierProvider.value(value: pluginSlots),
      Provider<PluginPageBuilderRegistry>(create: (_) {
        final builders = PluginPageBuilderRegistry();
        builders.register('declarative', declarativePageBuilder);
        builders.register('web-ui', pluginWebPageBuilder);
        return builders;
      }),
      ChangeNotifierProvider(create: (_) => ScheduleFocus()),
    ],
    child: const CoursiveApp(),
  ));
  unawaited(DeepLinks.start());
}

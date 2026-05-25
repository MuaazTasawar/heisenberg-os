import 'package:flutter/material.dart';
import '../services/api_service.dart';

class EmpireProvider extends ChangeNotifier {
  Map<String, dynamic>? empire;
  Map<String, dynamic>? threat;

  Future<void> loadState() async {
    final res = await ApiService.get('/game/state');
    if (res['success'] == true) {
      empire = res['data']['empire'];
      threat = res['data']['dea_threat'];
      notifyListeners();
    }
  }

  Future<void> cook(double effort) async {
    final res = await ApiService.post('/empire/cook', {'effort': effort});
    if (res['success'] == true) {
      empire = res['data'];
      notifyListeners();
    }
  }
}
import 'package:flutter/material.dart';
import '../services/api_service.dart';

class LegalScreen extends StatefulWidget {
  const LegalScreen({super.key});
  @override
  State<LegalScreen> createState() => _LegalScreenState();
}

class _LegalScreenState extends State<LegalScreen> {
  final chargeCtrl = TextEditingController();
  final evidenceCtrl = TextEditingController();
  String advice = '';
  bool loading = false;

  Future<void> _consult() async {
    setState(() => loading = true);
    final res = await ApiService.post('/legal/consult', {
      'charge': chargeCtrl.text,
      'evidence': evidenceCtrl.text,
      'player_context': 'Mobile user',
    });
    setState(() {
      advice = res['data']?['advice'] ?? 'Saul is unavailable.';
      loading = false;
    });
  }

  InputDecoration _input(String hint) => InputDecoration(
    hintText: hint, hintStyle: const TextStyle(color: Colors.grey),
    enabledBorder: const OutlineInputBorder(borderSide: BorderSide(color: Colors.grey)),
    focusedBorder: const OutlineInputBorder(borderSide: BorderSide(color: Color(0xFFFFD700))),
  );

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(backgroundColor: Colors.black, title: const Text('Better Call Saul', style: TextStyle(color: Color(0xFFFFD700)))),
      body: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: Column(children: [
          const Text('505-503-4455 — I fight for you!', style: TextStyle(color: Colors.grey)),
          const SizedBox(height: 20),
          TextField(controller: chargeCtrl, style: const TextStyle(color: Colors.white), decoration: _input('Charge')),
          const SizedBox(height: 12),
          TextField(controller: evidenceCtrl, maxLines: 3, style: const TextStyle(color: Colors.white), decoration: _input('Evidence against you')),
          const SizedBox(height: 16),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton(
              onPressed: loading ? null : _consult,
              style: ElevatedButton.styleFrom(backgroundColor: const Color(0xFFFFD700), foregroundColor: Colors.black, padding: const EdgeInsets.symmetric(vertical: 16)),
              child: Text(loading ? 'Saul is thinking...' : 'CONSULT SAUL', style: const TextStyle(fontWeight: FontWeight.bold, letterSpacing: 2)),
            ),
          ),
          if (advice.isNotEmpty) ...[
            const SizedBox(height: 20),
            Container(
              width: double.infinity,
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(border: Border.all(color: const Color(0xFFFFD700).withOpacity(0.5))),
              child: Text(advice, style: const TextStyle(color: Colors.white70, height: 1.6)),
            ),
          ],
        ]),
      ),
    );
  }
}
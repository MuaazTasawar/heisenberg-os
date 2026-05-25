import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/empire_provider.dart';

class DEAScreen extends StatelessWidget {
  const DEAScreen({super.key});

  @override
  Widget build(BuildContext context) {
    final threat = context.watch<EmpireProvider>().threat;
    final score = threat?['threat_score'] ?? 0;
    final color = score > 75 ? Colors.red : score > 50 ? Colors.orange : score > 25 ? Colors.yellow : Colors.green;

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(backgroundColor: Colors.black, title: const Text('DEA Status', style: TextStyle(color: Color(0xFFFFD700)))),
      body: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            const Text('OPERATION HEISENBERG', style: TextStyle(color: Colors.grey, letterSpacing: 4)),
            const SizedBox(height: 32),
            Container(
              padding: const EdgeInsets.all(32),
              decoration: BoxDecoration(border: Border.all(color: Colors.red.withOpacity(0.5))),
              child: Column(children: [
                Text('${score.toStringAsFixed(0)}%', style: TextStyle(color: color, fontSize: 72, fontWeight: FontWeight.bold)),
                const Text('THREAT SCORE', style: TextStyle(color: Colors.grey, letterSpacing: 4)),
                const SizedBox(height: 16),
                Text('Agent: ${threat?['lead_agent'] ?? 'Unknown'}', style: const TextStyle(color: Colors.white70)),
                Text('Evidence: ${threat?['evidence_count'] ?? 0} items', style: const TextStyle(color: Colors.white70)),
              ]),
            ),
          ],
        ),
      ),
    );
  }
}
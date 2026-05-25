import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/empire_provider.dart';

class EmpireScreen extends StatefulWidget {
  const EmpireScreen({super.key});
  @override
  State<EmpireScreen> createState() => _EmpireScreenState();
}

class _EmpireScreenState extends State<EmpireScreen> {
  double effort = 5;
  String msg = '';

  @override
  Widget build(BuildContext context) {
    final emp = context.watch<EmpireProvider>().empire;
    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(backgroundColor: Colors.black, title: const Text('The Cook Lab', style: TextStyle(color: Color(0xFFFFD700)))),
      body: emp == null ? const Center(child: CircularProgressIndicator()) : Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          children: [
            Row(children: [
              _stat('Purity', '${emp['purity_rating']?.toStringAsFixed(1)}%', Colors.blue),
              const SizedBox(width: 12),
              _stat('Yield kg', '${emp['batch_yield']?.toStringAsFixed(2)}', Colors.green),
              const SizedBox(width: 12),
              _stat('Heat', '${emp['heat_level']?.toStringAsFixed(1)}%', Colors.red),
            ]),
            const SizedBox(height: 32),
            Text('Effort Level: ${effort.toStringAsFixed(0)}', style: const TextStyle(color: Colors.white)),
            Slider(value: effort, min: 1, max: 10, divisions: 9, activeColor: const Color(0xFFFFD700), onChanged: (v) => setState(() => effort = v)),
            const SizedBox(height: 16),
            SizedBox(
              width: double.infinity,
              child: ElevatedButton(
                onPressed: () async {
                  await context.read<EmpireProvider>().cook(effort);
                  setState(() => msg = 'Batch cooked. Stay sharp.');
                },
                style: ElevatedButton.styleFrom(backgroundColor: const Color(0xFFFFD700), foregroundColor: Colors.black, padding: const EdgeInsets.symmetric(vertical: 16)),
                child: const Text('START COOK', style: TextStyle(fontWeight: FontWeight.bold, letterSpacing: 2)),
              ),
            ),
            if (msg.isNotEmpty) Padding(padding: const EdgeInsets.only(top: 16), child: Text(msg, style: const TextStyle(color: Colors.green))),
          ],
        ),
      ),
    );
  }

  Widget _stat(String label, String value, Color color) => Expanded(
    child: Container(
      decoration: BoxDecoration(border: Border.all(color: color.withOpacity(0.5))),
      padding: const EdgeInsets.all(12),
      child: Column(children: [
        Text(label, style: const TextStyle(color: Colors.grey, fontSize: 11)),
        Text(value, style: TextStyle(color: color, fontSize: 16, fontWeight: FontWeight.bold)),
      ]),
    ),
  );
}
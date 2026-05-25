import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/empire_provider.dart';
import '../widgets/stat_card.dart';
import 'empire_screen.dart';
import 'dea_screen.dart';
import 'legal_screen.dart';

class DashboardScreen extends StatefulWidget {
  const DashboardScreen({super.key});
  @override
  State<DashboardScreen> createState() => _DashboardScreenState();
}

class _DashboardScreenState extends State<DashboardScreen> {
  @override
  void initState() {
    super.initState();
    context.read<EmpireProvider>().loadState();
  }

  @override
  Widget build(BuildContext context) {
    final prov = context.watch<EmpireProvider>();
    final emp = prov.empire;
    final threat = prov.threat;

    return Scaffold(
      backgroundColor: Colors.black,
      appBar: AppBar(backgroundColor: Colors.black, title: const Text('HEISENBERG OS', style: TextStyle(color: Color(0xFFFFD700), letterSpacing: 4, fontWeight: FontWeight.bold))),
      body: emp == null
          ? const Center(child: CircularProgressIndicator(color: Color(0xFFFFD700)))
          : Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  GridView.count(
                    crossAxisCount: 2,
                    shrinkWrap: true,
                    crossAxisSpacing: 12,
                    mainAxisSpacing: 12,
                    childAspectRatio: 2.2,
                    children: [
                      StatCard(label: 'Purity', value: '${emp['purity_rating']?.toStringAsFixed(1)}%', color: Colors.blue),
                      StatCard(label: 'Dirty Money', value: '\$${emp['dirty_money']?.toStringAsFixed(0)}', color: Colors.red),
                      StatCard(label: 'Clean Money', value: '\$${emp['clean_money']?.toStringAsFixed(0)}', color: Colors.green),
                      StatCard(label: 'DEA Heat', value: '${threat?['threat_score']?.toStringAsFixed(0) ?? 0}%', color: Colors.orange),
                    ],
                  ),
                  const SizedBox(height: 24),
                  const Text('OPERATIONS', style: TextStyle(color: Colors.grey, letterSpacing: 4, fontSize: 12)),
                  const SizedBox(height: 12),
                  ...[
                    ('⚗️  Cook Lab', const EmpireScreen()),
                    ('🚔  DEA Status', const DEAScreen()),
                    ('⚖️  Call Saul', const LegalScreen()),
                  ].map((item) => Padding(
                    padding: const EdgeInsets.only(bottom: 8),
                    child: ListTile(
                      tileColor: const Color(0xFF111111),
                      title: Text(item.$1, style: const TextStyle(color: Colors.white, fontWeight: FontWeight.bold)),
                      trailing: const Icon(Icons.chevron_right, color: Color(0xFFFFD700)),
                      onTap: () => Navigator.push(context, MaterialPageRoute(builder: (_) => item.$2)),
                    ),
                  )),
                ],
              ),
            ),
    );
  }
}
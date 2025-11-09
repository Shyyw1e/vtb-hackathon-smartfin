import 'package:fl_chart/fl_chart.dart';
import 'package:flutter/material.dart';
import 'package:smart_fin/classes/transaction_widget.dart';
import 'package:smart_fin/services/api_service.dart';

class AnalysisPage extends StatefulWidget {
  //final LoginResponse loginresponse;
  const AnalysisPage({super.key});

  @override
  State<AnalysisPage> createState() => _AnalysisPage();
}

class _AnalysisPage extends State<AnalysisPage> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        children: [
          Expanded(
            child: Align(
              alignment: AlignmentGeometry.center,

              child: Container(
                height: MediaQuery.sizeOf(context).height * 0.3,
                width: MediaQuery.sizeOf(context).width * 0.3,
                child: PieChart(
                  PieChartData(
                    sections: [
                      PieChartSectionData(
                        value: 40,
                        color: Colors.blue,
                        title: 'Транспорт',
                      ),
                      PieChartSectionData(
                        value: 60,
                        color: Colors.brown,
                        title: 'Продукты',
                      ),
                      PieChartSectionData(
                        value: 60,
                        color: Colors.brown,
                        title: 'Аренда жилья',
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
          // Expanded(
          //   child: TransactionsWidget(
          //     accessToken: '',
          //     selectedBank: 'vbank', // или 'abank', 'sbank'
          //   ),
          // ),
        ],
      ),
    );
  }
}

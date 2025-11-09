import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:smart_fin/pages/auth_page.dart';

void main() {
  runApp(const MyApp());
}

class MyApp extends StatelessWidget {
  const MyApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(home: const MyHomePage());
  }
}

class MyHomePage extends StatefulWidget {
  const MyHomePage({super.key});

  @override
  State<MyHomePage> createState() => _MyHomePageState();
}

class _MyHomePageState extends State<MyHomePage> {
  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: Color.fromARGB(255, 64, 64, 64),
      body: Column(
        children: [
          SizedBox(height: MediaQuery.of(context).size.height * 0.07),
          Row(
            children: [
              SizedBox(width: MediaQuery.of(context).size.width * 0.08),
              Container(
                height: MediaQuery.of(context).size.height * 0.06,
                width: MediaQuery.of(context).size.width * 0.30,

                decoration: BoxDecoration(
                  borderRadius: BorderRadius.all(Radius.circular(15)),
                  color: Color.fromARGB(255, 190, 190, 190),
                ),
                child: Align(
                  alignment: AlignmentGeometry.center,
                  child: Text("Smart", style: TextStyle(fontSize: 34)),
                ),
              ),
              SizedBox(width: MediaQuery.of(context).size.width * 0.02),
              Container(
                child: Text(
                  "Fin",
                  style: TextStyle(
                    fontSize: 34,
                    color: Color.fromARGB(255, 190, 190, 190),
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: MediaQuery.of(context).size.height * 0.13),
          SvgPicture.asset(
            'assets/images/homepicture.svg',
            height: MediaQuery.of(context).size.height * 0.33,
            width: MediaQuery.of(context).size.width * 0.33,
          ),
          SizedBox(height: MediaQuery.of(context).size.height * 0.05),
          Expanded(
            child: Align(
              alignment: AlignmentGeometry.topCenter,
              child: Text(
                "Твоя жизнь - твои решения!",
                style: TextStyle(
                  fontSize: 30,
                  color: Color.fromARGB(255, 2, 171, 71),
                ),
              ),
            ),
          ),
          Expanded(
            child: Align(
              alignment: AlignmentGeometry.center,
              child: MaterialButton(
                minWidth: MediaQuery.of(context).size.width * 0.8,
                height: MediaQuery.of(context).size.height * 0.05,
                onPressed: () {
                  Navigator.push(
                    context,
                    MaterialPageRoute(builder: (context) => AuthPage()),
                  );
                },
                child: Text("Начать!"),
                color: Color.fromARGB(255, 190, 190, 190),
                shape: RoundedRectangleBorder(
                  borderRadius: BorderRadius.circular(15),
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

import 'package:flutter/material.dart';
import 'package:smart_fin/classes/auth_obj.dart';
import 'package:smart_fin/classes/sign_up_class.dart';
import 'package:smart_fin/pages/analysis_page.dart';
import 'package:smart_fin/services/api_service.dart';
import 'package:smart_fin/services/api_service_for_sign_up.dart';

class SignUpPage extends StatefulWidget {
  const SignUpPage({super.key});

  @override
  State<SignUpPage> createState() => _SignUpPage();
}

class _SignUpPage extends State<SignUpPage> {
  final auth_object = SignUp(name: '', password: '', email: '');

  final TextEditingController _passwordController = TextEditingController();
  final TextEditingController _emailController = TextEditingController();
  final TextEditingController _nameColntroller = TextEditingController();

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: Column(
        children: [
          SizedBox(height: MediaQuery.of(context).size.height * 0.4),
          TextField(
            controller: _passwordController,
            decoration: InputDecoration(
              labelText: 'Пароль',
              hintText: 'Введите ваш пароль',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.password),
            ),
          ),

          SizedBox(height: 20),

          // Поле для email
          TextField(
            controller: _emailController,
            decoration: InputDecoration(
              labelText: 'Email',
              hintText: 'Введите ваш email',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.email),
            ),
            keyboardType: TextInputType.emailAddress,
          ),

          SizedBox(height: 30),

          TextField(
            controller: _nameColntroller,
            decoration: InputDecoration(
              labelText: 'Name',
              hintText: 'Введите ваше имя',
              border: OutlineInputBorder(),
              prefixIcon: Icon(Icons.person),
            ),
            keyboardType: TextInputType.emailAddress,
          ),

          SizedBox(height: 30),

          // Кнопка сохранения
          ElevatedButton(
            onPressed: _saveUser,
            style: ElevatedButton.styleFrom(
              backgroundColor: Colors.blue,
              foregroundColor: Colors.white,
              padding: EdgeInsets.symmetric(horizontal: 40, vertical: 15),
            ),
            child: Text(
              'Сохранить пользователя',
              style: TextStyle(fontSize: 16),
            ),
          ),
        ],
      ),
    );
  }

  void _saveUser() async {
    // Присваиваем значения полям класса
    setState(() {
      auth_object.password = _passwordController.text;
      auth_object.email = _emailController.text;
      auth_object.name = _nameColntroller.text;
    });
    final succses = await ApiServiceForSignUp.createUser(auth_object);

    if (auth_object.password != '' && auth_object.email != '') {
      // Показываем сообщение об успехе
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Данные пользователя сохранены!'),
          backgroundColor: Colors.green,
          duration: Duration(seconds: 2),
        ),
      );
      await Future.delayed(Duration(seconds: 1));
      // Navigator.push(
      //   context,
      //   MaterialPageRoute(
      //     builder: (context) => AnalysisPage(
      //       loginresponse: LoginResponse(
      //         accessToken: '',
      //         refreshToken: '',
      //         expiresIn: 0,
      //       ),
      //     ),
      //   ),
      // );
    } else {
      ScaffoldMessenger.of(context).showSnackBar(
        SnackBar(
          content: Text('Некорректный ввод данных'),
          backgroundColor: Colors.red,
          duration: Duration(seconds: 2),
        ),
      );
    }
  }

  @override
  void dispose() {
    // Очищаем контроллеры
    _passwordController.dispose();
    _emailController.dispose();
    super.dispose();
  }
}

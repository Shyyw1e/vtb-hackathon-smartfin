import 'package:flutter/material.dart';
import 'package:http/http.dart' as http;
import 'dart:convert';

class TransactionsWidget extends StatefulWidget {
  final String accessToken;
  final String selectedBank;

  const TransactionsWidget({
    Key? key,
    required this.accessToken,
    required this.selectedBank,
  }) : super(key: key);

  @override
  State<TransactionsWidget> createState() => _TransactionsWidgetState();
}

class _TransactionsWidgetState extends State<TransactionsWidget> {
  List<Transaction> _transactions = [];
  bool _isLoading = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _loadTransactions();
  }

  @override
  void didUpdateWidget(TransactionsWidget oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.selectedBank != widget.selectedBank) {
      _loadTransactions();
    }
  }

  Future<void> _loadTransactions() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final transactions = await _fetchTransactions();
      setState(() {
        _transactions = transactions;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  Future<List<Transaction>> _fetchTransactions() async {
    final sinceDate = DateTime.now().subtract(const Duration(days: 30));
    final sinceString =
        '${sinceDate.year}-${sinceDate.month.toString().padLeft(2, '0')}-${sinceDate.day.toString().padLeft(2, '0')}';

    final uri = Uri.parse('http://localhost:8080/api/v1/transactions').replace(
      queryParameters: {
        'bank': widget.selectedBank,
        'since': sinceString,
        'limit': '500',
      },
    );

    final response = await http.get(
      uri,
      headers: {'Authorization': 'Bearer ${widget.accessToken}'},
    );

    if (response.statusCode == 200) {
      final jsonData = json.decode(response.body);
      final transactionsList = jsonData['transactions'] as List;
      return transactionsList
          .map((item) => Transaction.fromJson(item))
          .toList();
    } else {
      throw Exception('Failed to load transactions: ${response.statusCode}');
    }
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      height: MediaQuery.of(context).size.height * 0.5, // Пол страницы
      padding: const EdgeInsets.all(16.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                'Транзакции (${widget.selectedBank})',
                style: const TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.bold,
                ),
              ),
              const SizedBox(width: 10),
              if (_isLoading)
                const SizedBox(
                  width: 20,
                  height: 20,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              const Spacer(),
              IconButton(
                icon: const Icon(Icons.refresh),
                onPressed: _isLoading ? null : _loadTransactions,
              ),
            ],
          ),
          const SizedBox(height: 16),
          Expanded(child: _buildContent()),
        ],
      ),
    );
  }

  Widget _buildContent() {
    if (_isLoading && _transactions.isEmpty) {
      return const Center(child: CircularProgressIndicator());
    }

    if (_error != null) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Text('Ошибка загрузки', style: TextStyle(color: Colors.red[700])),
            const SizedBox(height: 8),
            ElevatedButton(
              onPressed: _loadTransactions,
              child: const Text('Повторить'),
            ),
          ],
        ),
      );
    }

    if (_transactions.isEmpty) {
      return const Center(child: Text('Нет транзакций за выбранный период'));
    }

    return ListView.separated(
      itemCount: _transactions.length,
      separatorBuilder: (context, index) => const Divider(height: 1),
      itemBuilder: (context, index) {
        final transaction = _transactions[index];
        return TransactionItem(transaction: transaction);
      },
    );
  }
}

class TransactionItem extends StatelessWidget {
  final Transaction transaction;

  const TransactionItem({Key? key, required this.transaction})
    : super(key: key);

  @override
  Widget build(BuildContext context) {
    final amount = transaction.amount.amountMinor / 100.0;
    final isDebit = transaction.direction == 'debit';
    final color = isDebit ? Colors.red : Colors.green;
    final sign = isDebit ? '-' : '+';

    return ListTile(
      contentPadding: const EdgeInsets.symmetric(horizontal: 8.0),
      leading: Container(
        width: 40,
        height: 40,
        decoration: BoxDecoration(
          color: Colors.grey[200],
          borderRadius: BorderRadius.circular(8),
        ),
        child: Icon(
          _getTransactionIcon(transaction.merchant),
          color: Colors.grey[600],
        ),
      ),
      title: Text(
        transaction.merchant,
        style: const TextStyle(fontWeight: FontWeight.w500),
        overflow: TextOverflow.ellipsis,
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          if (transaction.description.isNotEmpty)
            Text(
              transaction.description,
              overflow: TextOverflow.ellipsis,
              style: const TextStyle(fontSize: 12),
            ),
          Text(
            _formatDate(transaction.bookedAt),
            style: TextStyle(fontSize: 12, color: Colors.grey[600]),
          ),
        ],
      ),
      trailing: Column(
        mainAxisAlignment: MainAxisAlignment.center,
        crossAxisAlignment: CrossAxisAlignment.end,
        children: [
          Text(
            '$sign${amount.toStringAsFixed(2)} ${transaction.amount.currency}',
            style: TextStyle(
              color: color,
              fontWeight: FontWeight.bold,
              fontSize: 16,
            ),
          ),
          Text(
            transaction.id,
            style: TextStyle(fontSize: 10, color: Colors.grey[500]),
          ),
        ],
      ),
    );
  }

  IconData _getTransactionIcon(String merchant) {
    final lowerMerchant = merchant.toLowerCase();
    if (lowerMerchant.contains('yandex')) return Icons.stream;
    if (lowerMerchant.contains('food') || lowerMerchant.contains('еда'))
      return Icons.restaurant;
    if (lowerMerchant.contains('fuel') || lowerMerchant.contains('заправка'))
      return Icons.local_gas_station;
    if (lowerMerchant.contains('market') || lowerMerchant.contains('магазин'))
      return Icons.shopping_cart;
    return Icons.receipt;
  }

  String _formatDate(String dateString) {
    try {
      final date = DateTime.parse(dateString).toLocal();
      return '${date.day.toString().padLeft(2, '0')}.${date.month.toString().padLeft(2, '0')}.${date.year} ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
    } catch (e) {
      return dateString;
    }
  }
}

class Transaction {
  final String id;
  final String accountId;
  final Amount amount;
  final String direction;
  final String merchant;
  final String description;
  final String bank;
  final String bookedAt;

  Transaction({
    required this.id,
    required this.accountId,
    required this.amount,
    required this.direction,
    required this.merchant,
    required this.description,
    required this.bank,
    required this.bookedAt,
  });

  factory Transaction.fromJson(Map<String, dynamic> json) {
    return Transaction(
      id: json['id'] ?? '',
      accountId: json['account_id'] ?? '',
      amount: Amount.fromJson(json['amount']),
      direction: json['direction'] ?? '',
      merchant: json['merchant'] ?? '',
      description: json['description'] ?? '',
      bank: json['bank'] ?? '',
      bookedAt: json['booked_at'] ?? '',
    );
  }
}

class Amount {
  final int amountMinor;
  final String currency;

  Amount({required this.amountMinor, required this.currency});

  factory Amount.fromJson(Map<String, dynamic> json) {
    return Amount(
      amountMinor: json['amount_minor'] ?? 0,
      currency: json['currency'] ?? '',
    );
  }
}
